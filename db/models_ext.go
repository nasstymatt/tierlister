package db

import (
	"context"
	"nastymatt/tierlister/db/sqlc"
	"path"
	"strings"
)

type ImageView struct {
	ID               int64
	ImageName        string
	ImageURL         string
	OriginalFilename string
	Position         float64
}

type TierWithImages struct {
	sqlc.GetTiersByTierlistRow
	Images []ImageView
}

func GetTiersWithImages(ctx context.Context, q sqlc.Queries, tierlistID int64) ([]TierWithImages, error) {
	tiers, err := q.GetTiersByTierlist(ctx, tierlistID)
	if err != nil {
		return nil, err
	}

	imageRows, err := q.GetImagesByTierlist(ctx, tierlistID)
	if err != nil {
		return nil, err
	}

	imagesByTier := make(map[int64][]ImageView, len(tiers))
	var allImages []ImageView
	for _, row := range imageRows {
		img := ImageToImageView(sqlc.Image{
			ID:               row.ID,
			ImagePath:        row.ImagePath,
			OriginalFilename: row.OriginalFilename,
		})
		img.Position = row.Position

		imagesByTier[row.TierID] = append(imagesByTier[row.TierID], img)
		allImages = append(allImages, img)
	}

	result := make([]TierWithImages, 0, len(tiers))
	for _, tier := range tiers {
		result = append(result, TierWithImages{
			GetTiersByTierlistRow: tier,
			Images:                imagesByTier[tier.ID],
		})
	}

	return result, nil
}

func GetImages(ctx context.Context, q sqlc.Queries) ([]ImageView, error) {
	images, err := q.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]ImageView, 0, len(images))
	for _, image := range images {
		result = append(result, ImageToImageView(image))
	}

	return result, nil
}

func ImageToImageView(image sqlc.Image) ImageView {
	return ImageView{
		ID:               image.ID,
		ImageURL:         strings.Replace(image.ImagePath, "data/images", "/uploads/images", 1),
		ImageName:        path.Base(image.ImagePath),
		OriginalFilename: image.OriginalFilename,
	}
}

func MoveImage(ctx context.Context, q sqlc.Queries, tierlistID, tierID, imageID int64, position int) error {
	rows, err := q.GetImagesByTier(ctx, sqlc.GetImagesByTierParams{
		TierlistID: tierlistID,
		TierID:     tierID,
	})
	if err != nil {
		return err
	}

	// Build ordered list of IDs in the target tier, excluding the image being moved.
	ids := make([]int64, 0, len(rows)+1)
	for _, row := range rows {
		if row.ImageID != imageID {
			ids = append(ids, row.ImageID)
		}
	}

	// Clamp 0-indexed position (Alpine Sort passes newIndex, starting at 0).
	idx := min(max(0, position), len(ids))

	// Insert the moved image at the target index.
	ids = append(ids, 0)
	copy(ids[idx+1:], ids[idx:])
	ids[idx] = imageID

	// UPSERT the moved image (handles both new placement and tier change).
	if err := q.UpsertTierImage(ctx, sqlc.UpsertTierImageParams{
		TierlistID: tierlistID,
		TierID:     tierID,
		ImageID:    imageID,
		Position:   float64(idx + 1),
	}); err != nil {
		return err
	}

	// Renumber all other images in the tier to clean float64 positions.
	for i, id := range ids {
		if id == imageID {
			continue
		}
		if err := q.UpdateTierImagePosition(ctx, sqlc.UpdateTierImagePositionParams{
			TierlistID: tierlistID,
			ImageID:    id,
			Position:   float64(i + 1),
		}); err != nil {
			return err
		}
	}

	return nil
}

func GetAvailableTierlistImages(ctx context.Context, q sqlc.Queries, tierlistID int64) ([]ImageView, error) {
	images, err := q.GetAvailableTierlistImages(ctx, tierlistID)
	if err != nil {
		return nil, err
	}

	result := make([]ImageView, 0, len(images))
	for _, image := range images {
		result = append(result, ImageToImageView(image))
	}

	return result, nil
}

-- name: ListTierlists :many
SELECT * FROM tierlists ORDER BY created_at;

-- name: CreateTierlist :one
INSERT INTO tierlists(title) VALUES(?)
RETURNING *;

-- name: GetTierlist :one
SELECT * FROM tierlists WHERE id = ?;

-- name: UpdateTierlist :exec
UPDATE tierlists SET title = ? WHERE id = ?;

-- name: DeleteTierlist :exec
DELETE FROM tierlists WHERE id = ?;

-- name: GetTiersByTierlist :many
SELECT id, title, color, position
FROM tiers
WHERE tierlist_id = ?
ORDER BY position ASC;

-- name: GetImagesByTierlist :many
SELECT
    ti.tier_id,
    ti.position,
    i.id,
    i.image_path,
    i.original_filename
FROM tier_images ti
JOIN images i ON i.id = ti.image_id
WHERE ti.tierlist_id = ?
ORDER BY ti.position ASC;

-- name: ListImages :many
SELECT * FROM images ORDER BY created_at DESC;

-- name: CreateImage :one
INSERT INTO images(image_path, original_filename) VALUES(?, ?)
RETURNING *;

-- name: GetAvailableTierlistImages :many
SELECT * FROM images i
WHERE NOT EXISTS (
    SELECT 1 FROM tier_images ti
    WHERE ti.image_id = i.id AND ti.tierlist_id = ?
)
ORDER BY created_at DESC;

-- name: UpsertTierImage :exec
INSERT INTO tier_images (tierlist_id, tier_id, image_id, position)
VALUES (?, ?, ?, ?)
ON CONFLICT (image_id, tierlist_id) DO UPDATE
SET tier_id = excluded.tier_id,
    position = excluded.position;

-- name: GetImagesByTier :many
SELECT image_id, position
FROM tier_images
WHERE tierlist_id = ? AND tier_id = ?
ORDER BY position ASC;

-- name: UpdateTierImagePosition :exec
UPDATE tier_images
SET position = ?
WHERE tierlist_id = ? AND image_id = ?;

-- name: DeleteTierImage :exec
DELETE FROM tier_images
WHERE tierlist_id = ? AND image_id = ?;

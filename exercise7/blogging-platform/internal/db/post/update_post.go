package post

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/talgat-ruby/exercises-go/exercise7/blogging-platform/internal/db/blog"
)

func (p *Post) UpdatePost(ctx context.Context, id_post int, req blog.PostRequest) (*blog.Post, error) {
	log := p.logger.With("method", "UpdatePost")

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		log.ErrorContext(ctx, "fail to begin transaction", "error", err)
		return nil, err
	}
	defer tx.Rollback()

	id_category, err := p.GetCategoryIDByName(ctx, tx, req.Category)
	if err != nil {
		log.ErrorContext(ctx, "failed to get category by name", "error", err)
		return nil, err
	}
	category := &blog.Category{
		ID:   id_category,
		Name: req.Category,
	}
	if id_category == 0 {
		category, err = p.InsertCat(ctx, tx, *category)
		if err != nil {
			log.ErrorContext(ctx, "failed to create category", "error", err)
			return nil, err
		}
	}

	id_category = category.ID
	post, err := p.UpdateInfoPost(ctx, tx, id_post, id_category, req)
	if err != nil {
		log.ErrorContext(ctx, "failed to update post", "error", err)
		return nil, err
	}
	post.Category = category

	tags, err := p.UpdateTags(ctx, tx, post.ID, req.Tags)
	if err != nil {
		log.ErrorContext(ctx, "failed to update tags", "error", err)
		return nil, err
	}
	post.Tags = tags

	if err := tx.Commit(); err != nil {
		log.ErrorContext(ctx, "fail to commit transaction", "error", err)
		return nil, err
	}

	log.InfoContext(ctx, "success update post")

	return post, nil
}

func (p *Post) UpdateInfoPost(ctx context.Context, tx *sql.Tx, id_post int, id_category int, req blog.PostRequest) (*blog.Post, error) {
	log := p.logger.With("method", "UpdatePost")

	var post blog.Post
	query := `UPDATE post SET id_category = $1 , title = $2, content = $3, updated_at = NOW() WHERE id = $4 RETURNING id, title, content, created_at, updated_at`
	err := tx.QueryRowContext(ctx, query, id_category, req.Title, req.Content, id_post).Scan(&post.ID, &post.Title, &post.Content, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		log.ErrorContext(ctx, "failed to update post", "error", err)
		return nil, err
	}

	return &post, nil
}

func (p *Post) UpdateTags(ctx context.Context, tx *sql.Tx, id_post int, tags []string) ([]*blog.Tag, error) {
	log := p.logger.With("method", "UpdatePost")

	query := `
	SELECT t.id, t.name
	FROM tag t
	JOIN post_tags pt ON pt.id_tag = t.id
	WHERE pt.id_post = $1
	`
	rows, err := tx.QueryContext(ctx, query, id_post)
	if err != nil {
		log.ErrorContext(ctx, "fail getting current tags for post", "error", err)
		return nil, err
	}
	defer rows.Close()

	currentTags := make(map[int]*blog.Tag)
	for rows.Next() {
		var tag blog.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			log.ErrorContext(ctx, "fail to scanning tag", "error", err)
			return nil, err
		}
		currentTags[tag.ID] = &tag
	}

	var newTagIDs []int
	for _, tagName := range tags {
		var tag blog.Tag

		err := tx.QueryRowContext(ctx, "SELECT id, name FROM tag WHERE name = $1", tagName).Scan(&tag.ID, &tag.Name)
		if err != nil && err != sql.ErrNoRows {
			log.ErrorContext(ctx, "fail to query tag", "error", err)
			return nil, err
		}

		if err == sql.ErrNoRows {

			err := tx.QueryRowContext(ctx, "INSERT INTO tag(name) VALUES($1) RETURNING id", tagName).Scan(&tag.ID)
			if err != nil {
				log.ErrorContext(ctx, "fail to insert new tag", "error", err)
				return nil, err
			}
		}

		newTagIDs = append(newTagIDs, tag.ID)
	}

	for tagID, _ := range currentTags {
		if !contains(newTagIDs, tagID) {
			_, err := tx.ExecContext(ctx, "DELETE FROM post_tags WHERE id_post = $1 AND id_tag = $2", id_post, tagID)
			if err != nil {
				log.ErrorContext(ctx, fmt.Sprintf("fail to delete tag %d from post %d", tagID, id_post), "error", err)
				return nil, err
			}
		}
	}

	for _, tagID := range newTagIDs {

		var count int
		err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM post_tags WHERE id_post = $1 AND id_tag = $2", id_post, tagID).Scan(&count)
		if err != nil {
			log.ErrorContext(ctx, fmt.Sprintf("fail to checking if post is already linked with tag: %d", tagID), "error", err)
			return nil, err
		}
		if count == 0 {
			_, err := tx.ExecContext(ctx, "INSERT INTO post_tags(id_post, id_tag) VALUES($1, $2)", id_post, tagID)
			if err != nil {
				log.ErrorContext(ctx, fmt.Sprintf("fail to insert post_tag for post %d and tag %d", id_post, tagID), "error", err)
				return nil, err
			}
		}
	}

	query = `
	SELECT t.id, t.name
	FROM tag t
	JOIN post_tags pt ON pt.id_tag = t.id
	WHERE pt.id_post = $1
	`
	rows, err = tx.QueryContext(ctx, query, id_post)
	if err != nil {
		log.ErrorContext(ctx, fmt.Sprintf("fail getting updated tags for post %d", id_post), "error", err)
		return nil, err
	}
	defer rows.Close()

	var updatedTags []*blog.Tag
	for rows.Next() {
		var tag blog.Tag
		if err := rows.Scan(&tag.ID, &tag.Name); err != nil {
			log.ErrorContext(ctx, "fail scanning updated tag", "error", err)
			return nil, err
		}
		updatedTags = append(updatedTags, &tag)
	}

	return updatedTags, nil
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

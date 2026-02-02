package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Label represents a tag/label that can be attached to articles.
type Label struct {
	gorm.Model
	Name string `gorm:"size:64;uniqueIndex;not null" json:"name"`
}

// Article represents a blog/article post.
type Article struct {
	gorm.Model
	Title       string    `gorm:"size:255;not null" json:"title"`
	Body        string    `gorm:"type:text;not null" json:"body"`
	Author      string    `gorm:"size:64;not null;index" json:"author"`
	PublishedAt time.Time `gorm:"autoCreateTime" json:"published_at"`
	// IdDeleted indicates whether the article is deleted (soft flag controlled by our app).
	IdDeleted bool `gorm:"column:id_deleted;default:false;not null" json:"id_deleted"`
	// Tags many-to-many relationship
	Tags []Label `gorm:"many2many:article_labels;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"tags"`
	// TagsCached stores comma-separated labels for quick reads (nullable)
	TagsCached string `gorm:"size:512;column:tags_cached;default:''" json:"tags_cached"`
}

// TableName returns the DB table name.
func (Article) TableName() string {
	return "articles"
}

// ListLabels returns all defined labels ordered by name
func ListLabels() ([]Label, error) {
	if DB == nil {
		return nil, gorm.ErrInvalidDB
	}
	var ls []Label
	if err := DB.Order("name asc").Find(&ls).Error; err != nil {
		return nil, err
	}
	return ls, nil
}

// CreateArticle creates a new article and returns it.
func CreateArticle(title, body, author string, tags []string) (*Article, error) {
	if DB == nil {
		return nil, gorm.ErrInvalidDB
	}
	a := &Article{Title: title, Body: body, Author: author, PublishedAt: time.Now()}
	// create/find labels and associate
	var labelObjs []Label
	for _, t := range tags {
		name := strings.TrimSpace(t)
		if name == "" {
			continue
		}
		var l Label
		if err := DB.Where("name = ?", name).First(&l).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				l = Label{Name: name}
				if err := DB.Create(&l).Error; err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
		labelObjs = append(labelObjs, l)
	}
	if err := DB.Create(a).Error; err != nil {
		return nil, err
	}
	if len(labelObjs) > 0 {
		if err := DB.Model(a).Association("Tags").Replace(labelObjs); err != nil {
			return nil, err
		}
		// update cached string
		var names []string
		for _, l := range labelObjs {
			names = append(names, l.Name)
		}
		a.TagsCached = strings.Join(names, ",")
		if err := DB.Save(a).Error; err != nil {
			return nil, err
		}
	}
	return a, nil
}

// GetArticle retrieves an article by ID.
func GetArticle(id uint) (*Article, error) {
	if DB == nil {
		return nil, gorm.ErrInvalidDB
	}
	var a Article
	if err := DB.Preload("Tags").First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteArticle marks an article as deleted (IdDeleted=true) if author matches.
func DeleteArticle(id uint, username string) error {
	if DB == nil {
		return gorm.ErrInvalidDB
	}
	var a Article
	if err := DB.First(&a, id).Error; err != nil {
		return err
	}
	if a.Author != username {
		return gorm.ErrInvalidData
	}
	// mark as deleted
	a.IdDeleted = true
	if err := DB.Save(&a).Error; err != nil {
		return err
	}
	return nil
}

// UpdateArticle updates title and body if author matches (already added earlier)

// ListArticles returns paginated articles ordered by created_at desc.
func ListArticles(offset, limit int, tag string) ([]Article, error) {
	if DB == nil {
		return nil, gorm.ErrInvalidDB
	}
	var as []Article
	q := DB.Model(&Article{}).Where("id_deleted = ?", false).Order("created_at desc").Offset(offset).Limit(limit)
	if tag != "" {
		// join with labels
		q = q.Joins("JOIN article_labels al ON al.article_id = articles.id").Joins("JOIN labels l ON l.id = al.label_id").Where("l.name = ?", tag)
	}
	if err := q.Preload("Tags").Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
}

// UpdateArticle updates title, body, and tags if author matches
func UpdateArticle(id uint, title, body, username string, tags []string) error {
	if DB == nil {
		return gorm.ErrInvalidDB
	}
	var a Article
	if err := DB.First(&a, id).Error; err != nil {
		return err
	}
	if a.Author != username {
		return gorm.ErrInvalidData
	}
	a.Title = title
	a.Body = body
	if err := DB.Save(&a).Error; err != nil {
		return err
	}

	// update tags/labels
	var labelObjs []Label
	for _, t := range tags {
		name := strings.TrimSpace(t)
		if name == "" {
			continue
		}
		var l Label
		if err := DB.Where("name = ?", name).First(&l).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				l = Label{Name: name}
				if err := DB.Create(&l).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
		labelObjs = append(labelObjs, l)
	}

	// replace tags association
	if err := DB.Model(&a).Association("Tags").Replace(labelObjs); err != nil {
		return err
	}

	// update cached string
	var names []string
	for _, l := range labelObjs {
		names = append(names, l.Name)
	}
	a.TagsCached = strings.Join(names, ",")
	if err := DB.Save(&a).Error; err != nil {
		return err
	}

	return nil
}

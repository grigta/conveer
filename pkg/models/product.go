package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Product struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Name        string                 `bson:"name" json:"name" validate:"required,min=1,max=200"`
	Description string                 `bson:"description" json:"description"`
	Category    string                 `bson:"category" json:"category" validate:"required"`
	Price       float64                `bson:"price" json:"price" validate:"required,min=0"`
	Currency    string                 `bson:"currency" json:"currency"`
	SKU         string                 `bson:"sku" json:"sku" validate:"required"`
	Barcode     string                 `bson:"barcode" json:"barcode"`
	Stock       int                    `bson:"stock" json:"stock" validate:"min=0"`
	Images      []ProductImage         `bson:"images" json:"images"`
	Attributes  map[string]interface{} `bson:"attributes" json:"attributes"`
	Tags        []string               `bson:"tags" json:"tags"`
	Status      ProductStatus          `bson:"status" json:"status"`
	Weight      float64                `bson:"weight" json:"weight"`
	Dimensions  Dimensions             `bson:"dimensions" json:"dimensions"`
	Metadata    map[string]interface{} `bson:"metadata" json:"metadata"`
	CreatedBy   primitive.ObjectID     `bson:"created_by" json:"created_by"`
	UpdatedBy   primitive.ObjectID     `bson:"updated_by" json:"updated_by"`
	CreatedAt   time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time              `bson:"updated_at" json:"updated_at"`
}

type ProductImage struct {
	URL       string `bson:"url" json:"url"`
	Alt       string `bson:"alt" json:"alt"`
	IsPrimary bool   `bson:"is_primary" json:"is_primary"`
}

type Dimensions struct {
	Length float64 `bson:"length" json:"length"`
	Width  float64 `bson:"width" json:"width"`
	Height float64 `bson:"height" json:"height"`
	Unit   string  `bson:"unit" json:"unit"`
}

type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
	ProductStatusDraft    ProductStatus = "draft"
	ProductStatusArchived ProductStatus = "archived"
)

type Category struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name" validate:"required"`
	Slug        string             `bson:"slug" json:"slug" validate:"required"`
	Description string             `bson:"description" json:"description"`
	ParentID    *primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id"`
	Image       string             `bson:"image" json:"image"`
	SortOrder   int                `bson:"sort_order" json:"sort_order"`
	IsActive    bool               `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}

type Inventory struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID     primitive.ObjectID `bson:"product_id" json:"product_id"`
	WarehouseID   primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	Quantity      int                `bson:"quantity" json:"quantity"`
	ReservedQty   int                `bson:"reserved_qty" json:"reserved_qty"`
	AvailableQty  int                `bson:"available_qty" json:"available_qty"`
	MinStock      int                `bson:"min_stock" json:"min_stock"`
	MaxStock      int                `bson:"max_stock" json:"max_stock"`
	ReorderPoint  int                `bson:"reorder_point" json:"reorder_point"`
	LastRestocked time.Time          `bson:"last_restocked" json:"last_restocked"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}

type PriceHistory struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID  primitive.ObjectID `bson:"product_id" json:"product_id"`
	OldPrice   float64            `bson:"old_price" json:"old_price"`
	NewPrice   float64            `bson:"new_price" json:"new_price"`
	Currency   string             `bson:"currency" json:"currency"`
	ChangedBy  primitive.ObjectID `bson:"changed_by" json:"changed_by"`
	ChangeDate time.Time          `bson:"change_date" json:"change_date"`
	Reason     string             `bson:"reason" json:"reason"`
}

type ProductReview struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID  primitive.ObjectID `bson:"product_id" json:"product_id"`
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	Rating     int                `bson:"rating" json:"rating" validate:"min=1,max=5"`
	Title      string             `bson:"title" json:"title"`
	Comment    string             `bson:"comment" json:"comment"`
	Images     []string           `bson:"images" json:"images"`
	IsVerified bool               `bson:"is_verified" json:"is_verified"`
	Helpful    int                `bson:"helpful" json:"helpful"`
	NotHelpful int                `bson:"not_helpful" json:"not_helpful"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time          `bson:"updated_at" json:"updated_at"`
}

type CreateProductRequest struct {
	Name        string                 `json:"name" validate:"required,min=1,max=200"`
	Description string                 `json:"description"`
	Category    string                 `json:"category" validate:"required"`
	Price       float64                `json:"price" validate:"required,min=0"`
	Currency    string                 `json:"currency"`
	SKU         string                 `json:"sku" validate:"required"`
	Barcode     string                 `json:"barcode"`
	Stock       int                    `json:"stock" validate:"min=0"`
	Images      []ProductImage         `json:"images"`
	Attributes  map[string]interface{} `json:"attributes"`
	Tags        []string               `json:"tags"`
	Weight      float64                `json:"weight"`
	Dimensions  Dimensions             `json:"dimensions"`
}

type UpdateProductRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Price       float64                `json:"price"`
	Stock       int                    `json:"stock"`
	Images      []ProductImage         `json:"images"`
	Attributes  map[string]interface{} `json:"attributes"`
	Tags        []string               `json:"tags"`
	Status      ProductStatus          `json:"status"`
	Weight      float64                `json:"weight"`
	Dimensions  Dimensions             `json:"dimensions"`
}

type ProductFilter struct {
	Category    string   `json:"category"`
	MinPrice    float64  `json:"min_price"`
	MaxPrice    float64  `json:"max_price"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	InStock     bool     `json:"in_stock"`
	SearchQuery string   `json:"search_query"`
	SortBy      string   `json:"sort_by"`
	SortOrder   string   `json:"sort_order"`
	Page        int      `json:"page"`
	Limit       int      `json:"limit"`
}
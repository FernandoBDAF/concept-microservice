package examples

import (
	"time"

	"github.com/FBDAF/microservices/services/common/models/domain"
)

// Product represents a product in the system
type Product struct {
	domain.VersionedBaseModel
	Name        string                 `json:"name" validate:"required"`
	Description string                 `json:"description" validate:"required"`
	SKU         string                 `json:"sku" validate:"required"`
	Price       domain.Money           `json:"price" validate:"required"`
	Category    string                 `json:"category" validate:"required"`
	Tags        []string               `json:"tags,omitempty"`
	Status      domain.Status          `json:"status" validate:"required"`
	Inventory   int                    `json:"inventory" validate:"required,min=0"`
	Images      []string               `json:"images,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// Validate validates the Product
func (p *Product) Validate() error {
	if err := p.Price.Validate(); err != nil {
		return err
	}
	if p.Inventory < 0 {
		return &domain.ValidationError{
			Field:   "inventory",
			Message: "inventory cannot be negative",
		}
	}
	return nil
}

// ProductCreateRequest represents a request to create a product
type ProductCreateRequest struct {
	Name        string                 `json:"name" validate:"required"`
	Description string                 `json:"description" validate:"required"`
	SKU         string                 `json:"sku" validate:"required"`
	Price       domain.Money           `json:"price" validate:"required"`
	Category    string                 `json:"category" validate:"required"`
	Tags        []string               `json:"tags,omitempty"`
	Inventory   int                    `json:"inventory" validate:"required,min=0"`
	Images      []string               `json:"images,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// ProductUpdateRequest represents a request to update a product
type ProductUpdateRequest struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	SKU         string                 `json:"sku,omitempty"`
	Price       domain.Money           `json:"price,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Inventory   int                    `json:"inventory,omitempty" validate:"omitempty,min=0"`
	Images      []string               `json:"images,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// ProductResponse represents a product response
type ProductResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	SKU         string                 `json:"sku"`
	Price       domain.Money           `json:"price"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags,omitempty"`
	Status      domain.Status          `json:"status"`
	Inventory   int                    `json:"inventory"`
	Images      []string               `json:"images,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// ToResponse converts a Product to a ProductResponse
func (p *Product) ToResponse() *ProductResponse {
	return &ProductResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		SKU:         p.SKU,
		Price:       p.Price,
		Category:    p.Category,
		Tags:        p.Tags,
		Status:      p.Status,
		Inventory:   p.Inventory,
		Images:      p.Images,
		Attributes:  p.Attributes,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// NewProductFromCreateRequest creates a Product from a ProductCreateRequest
func NewProductFromCreateRequest(req *ProductCreateRequest) *Product {
	return &Product{
		VersionedBaseModel: *domain.NewVersionedBaseModel(),
		Name:               req.Name,
		Description:        req.Description,
		SKU:                req.SKU,
		Price:              req.Price,
		Category:           req.Category,
		Tags:               req.Tags,
		Status:             *domain.NewStatus("active", "Product created"),
		Inventory:          req.Inventory,
		Images:             req.Images,
		Attributes:         req.Attributes,
	}
}

// Update updates a Product from a ProductUpdateRequest
func (p *Product) Update(req *ProductUpdateRequest) {
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.SKU != "" {
		p.SKU = req.SKU
	}
	if req.Price.Amount != 0 {
		p.Price = req.Price
	}
	if req.Category != "" {
		p.Category = req.Category
	}
	if req.Tags != nil {
		p.Tags = req.Tags
	}
	if req.Status != "" {
		p.Status = *domain.NewStatus(req.Status, "Product updated")
	}
	if req.Inventory != 0 {
		p.Inventory = req.Inventory
	}
	if req.Images != nil {
		p.Images = req.Images
	}
	if req.Attributes != nil {
		p.Attributes = req.Attributes
	}
	p.UpdatedAt = time.Now()
}

package examples

import (
	"time"

	"github.com/FBDAF/microservices/services/common/models/domain"
)

// Order represents an order in the system
type Order struct {
	domain.VersionedBaseModel
	CustomerID string              `json:"customer_id" validate:"required"`
	Items      []OrderItem         `json:"items" validate:"required,min=1"`
	Total      domain.Money        `json:"total" validate:"required"`
	Status     domain.Status       `json:"status" validate:"required"`
	Shipping   domain.Address      `json:"shipping" validate:"required"`
	Billing    domain.Address      `json:"billing" validate:"required"`
	Payment    PaymentInfo         `json:"payment" validate:"required"`
	Notes      string              `json:"notes,omitempty"`
	Metadata   domain.MetadataList `json:"metadata,omitempty"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ProductID  string                 `json:"product_id" validate:"required"`
	Quantity   int                    `json:"quantity" validate:"required,min=1"`
	Price      domain.Money           `json:"price" validate:"required"`
	Total      domain.Money           `json:"total" validate:"required"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// PaymentInfo represents payment information
type PaymentInfo struct {
	Method      string       `json:"method" validate:"required"`
	Status      string       `json:"status" validate:"required"`
	Amount      domain.Money `json:"amount" validate:"required"`
	Reference   string       `json:"reference,omitempty"`
	ProcessedAt time.Time    `json:"processed_at,omitempty"`
}

// Validate validates the Order
func (o *Order) Validate() error {
	if err := o.Total.Validate(); err != nil {
		return err
	}
	if err := o.Payment.Amount.Validate(); err != nil {
		return err
	}
	if len(o.Items) == 0 {
		return &domain.ValidationError{
			Field:   "items",
			Message: "order must have at least one item",
		}
	}
	return nil
}

// OrderCreateRequest represents a request to create an order
type OrderCreateRequest struct {
	CustomerID string              `json:"customer_id" validate:"required"`
	Items      []OrderItem         `json:"items" validate:"required,min=1"`
	Shipping   domain.Address      `json:"shipping" validate:"required"`
	Billing    domain.Address      `json:"billing" validate:"required"`
	Payment    PaymentInfo         `json:"payment" validate:"required"`
	Notes      string              `json:"notes,omitempty"`
	Metadata   domain.MetadataList `json:"metadata,omitempty"`
}

// OrderUpdateRequest represents a request to update an order
type OrderUpdateRequest struct {
	Status   string              `json:"status,omitempty"`
	Shipping domain.Address      `json:"shipping,omitempty"`
	Billing  domain.Address      `json:"billing,omitempty"`
	Payment  PaymentInfo         `json:"payment,omitempty"`
	Notes    string              `json:"notes,omitempty"`
	Metadata domain.MetadataList `json:"metadata,omitempty"`
}

// OrderResponse represents an order response
type OrderResponse struct {
	ID         string              `json:"id"`
	CustomerID string              `json:"customer_id"`
	Items      []OrderItem         `json:"items"`
	Total      domain.Money        `json:"total"`
	Status     domain.Status       `json:"status"`
	Shipping   domain.Address      `json:"shipping"`
	Billing    domain.Address      `json:"billing"`
	Payment    PaymentInfo         `json:"payment"`
	Notes      string              `json:"notes,omitempty"`
	Metadata   domain.MetadataList `json:"metadata,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// ToResponse converts an Order to an OrderResponse
func (o *Order) ToResponse() *OrderResponse {
	return &OrderResponse{
		ID:         o.ID,
		CustomerID: o.CustomerID,
		Items:      o.Items,
		Total:      o.Total,
		Status:     o.Status,
		Shipping:   o.Shipping,
		Billing:    o.Billing,
		Payment:    o.Payment,
		Notes:      o.Notes,
		Metadata:   o.Metadata,
		CreatedAt:  o.CreatedAt,
		UpdatedAt:  o.UpdatedAt,
	}
}

// NewOrderFromCreateRequest creates an Order from an OrderCreateRequest
func NewOrderFromCreateRequest(req *OrderCreateRequest) *Order {
	return &Order{
		VersionedBaseModel: *domain.NewVersionedBaseModel(),
		CustomerID:         req.CustomerID,
		Items:              req.Items,
		Total:              calculateTotal(req.Items),
		Status:             *domain.NewStatus("pending", "Order created"),
		Shipping:           req.Shipping,
		Billing:            req.Billing,
		Payment:            req.Payment,
		Notes:              req.Notes,
		Metadata:           req.Metadata,
	}
}

// Update updates an Order from an OrderUpdateRequest
func (o *Order) Update(req *OrderUpdateRequest) {
	if req.Status != "" {
		o.Status = *domain.NewStatus(req.Status, "Order updated")
	}
	if req.Shipping.Street != "" {
		o.Shipping = req.Shipping
	}
	if req.Billing.Street != "" {
		o.Billing = req.Billing
	}
	if req.Payment.Method != "" {
		o.Payment = req.Payment
	}
	if req.Notes != "" {
		o.Notes = req.Notes
	}
	if req.Metadata != nil {
		o.Metadata = req.Metadata
	}
	o.UpdatedAt = time.Now()
}

// calculateTotal calculates the total amount for an order
func calculateTotal(items []OrderItem) domain.Money {
	var total float64
	for _, item := range items {
		total += item.Total.Amount
	}
	return domain.Money{
		Amount:   total,
		Currency: items[0].Total.Currency,
	}
}

package main

import "sync"

// OrderRepository manages orders in-memory
type OrderRepository struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

type Order struct {
	ID         string
	ProductID  string
	Quantity   int32
	CustomerID string
	Status     string // PENDING, PROCESSING, SHIPPED, DELIVERED
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{
		orders: make(map[string]*Order),
	}
}

func (r *OrderRepository) CreateOrder(order *Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.ID] = order
	return nil
}

func (r *OrderRepository) GetOrder(orderID string) (*Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if order, exists := r.orders[orderID]; exists {
		return order, nil
	}
	return nil, nil
}

func (r *OrderRepository) UpdateOrderStatus(orderID string, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if order, exists := r.orders[orderID]; exists {
		order.Status = status
		return nil
	}
	return nil
}

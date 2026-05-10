package main

import (
	"sync"
)

// InventoryRepository manages inventory in-memory
type InventoryRepository struct {
	mu       sync.RWMutex
	stock    map[string]int32
	tracking map[string][]string // order_id -> status updates
}

func NewInventoryRepository() *InventoryRepository {
	return &InventoryRepository{
		stock:    make(map[string]int32),
		tracking: make(map[string][]string),
	}
}

func (r *InventoryRepository) Initialize() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Initialize with some sample stock
	r.stock["PROD001"] = 100
	r.stock["PROD002"] = 50
	r.stock["PROD003"] = 200
}

func (r *InventoryRepository) CheckStock(productID string, quantity int32) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stock, exists := r.stock[productID]
	return exists && stock >= quantity
}

func (r *InventoryRepository) DeductStock(productID string, quantity int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stock, exists := r.stock[productID]; exists {
		r.stock[productID] = stock - quantity
	}
	return nil
}

func (r *InventoryRepository) GetStock(productID string) int32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stock[productID]
}

func (r *InventoryRepository) AddTrackingStatus(orderID string, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tracking[orderID] = append(r.tracking[orderID], status)
}

func (r *InventoryRepository) GetTrackingStatuses(orderID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if statuses, exists := r.tracking[orderID]; exists {
		return statuses
	}
	return []string{}
}

package logic

import "math"

// CalculatePrice applies the business rules for Milestone 2
func CalculatePrice(costPrice float64, currentStock int) float64 {
    // Basic Rule: 20% Profit Margin
    margin := 1.20

    // Dynamic Logic: Scarcity Pricing
    // If stock is very low (e.g., < 5), increase margin by 15%
    if currentStock > 0 && currentStock < 5 {
        margin += 0.15
    }

    // Dynamic Logic: Clearance Pricing
    // If stock is too high (e.g., > 100), decrease margin by 10%
    if currentStock > 100 {
        margin -= 0.10
    }

    newPrice := costPrice * margin
    
    // Round to 2 decimal places for currency
    return math.Round(newPrice*100) / 100
}
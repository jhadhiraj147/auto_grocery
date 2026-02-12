"""Item catalog for grocery orders. Aligns with assignment's 5 aisles."""

ITEMS_BY_AISLE = {
    "Bread": [
        {"sku": "BREAD-WHITE", "name": "White Bread", "unit": "loaf"},
        {"sku": "BREAD-WHOLE", "name": "Whole Wheat Bread", "unit": "loaf"},
        {"sku": "BREAD-BAGEL", "name": "Bagels", "unit": "pack"},
        {"sku": "BREAD-WAFFLE", "name": "Waffles", "unit": "pack"},
        {"sku": "BREAD-CROISSANT", "name": "Croissants", "unit": "pack"},
    ],
    "Dairy": [
        {"sku": "DAIRY-MILK", "name": "Milk", "unit": "gallon"},
        {"sku": "DAIRY-CHEDDAR", "name": "Cheddar Cheese", "unit": "lb"},
        {"sku": "DAIRY-YOGURT", "name": "Yogurt", "unit": "oz"},
        {"sku": "DAIRY-BUTTER", "name": "Butter", "unit": "stick"},
        {"sku": "DAIRY-CREAM", "name": "Heavy Cream", "unit": "pint"},
    ],
    "Meat": [
        {"sku": "MEAT-CHICKEN", "name": "Chicken Breast", "unit": "lb"},
        {"sku": "MEAT-BEEF", "name": "Ground Beef", "unit": "lb"},
        {"sku": "MEAT-PORK", "name": "Pork Chops", "unit": "lb"},
        {"sku": "MEAT-SALMON", "name": "Salmon Fillet", "unit": "lb"},
        {"sku": "MEAT-BACON", "name": "Bacon", "unit": "pack"},
    ],
    "Produce": [
        {"sku": "PROD-TOMATO", "name": "Tomatoes", "unit": "lb"},
        {"sku": "PROD-ONION", "name": "Onions", "unit": "lb"},
        {"sku": "PROD-APPLE", "name": "Apples", "unit": "lb"},
        {"sku": "PROD-ORANGE", "name": "Oranges", "unit": "lb"},
        {"sku": "PROD-LETTUCE", "name": "Lettuce", "unit": "head"},
        {"sku": "PROD-CARROT", "name": "Carrots", "unit": "lb"},
        {"sku": "PROD-BANANA", "name": "Bananas", "unit": "lb"},
    ],
    "Party Supply": [
        {"sku": "PARTY-SODA", "name": "Soda", "unit": "12-pack"},
        {"sku": "PARTY-PLATES", "name": "Paper Plates", "unit": "pack"},
        {"sku": "PARTY-NAPKIN", "name": "Napkins", "unit": "pack"},
        {"sku": "PARTY-CHIPS", "name": "Chips", "unit": "bag"},
        {"sku": "PARTY-CUPS", "name": "Plastic Cups", "unit": "pack"},
        {"sku": "PARTY-CAKE", "name": "Party Cake", "unit": "slice"},
    ],
}

AISLE_TYPES = list(ITEMS_BY_AISLE.keys())

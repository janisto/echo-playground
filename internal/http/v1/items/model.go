package items

import (
	"slices"
	"time"

	"github.com/janisto/echo-playground/internal/platform/timeutil"
)

// Item represents a sample resource for pagination demonstration.
type Item struct {
	ID          string        `json:"id"          cbor:"id"          example:"item-001"`
	Name        string        `json:"name"        cbor:"name"        example:"Alpha Widget"`
	Category    string        `json:"category"    cbor:"category"    example:"electronics"`
	Price       Money         `json:"price"       cbor:"price"`
	InStock     bool          `json:"inStock"     cbor:"inStock"     example:"true"`
	CreatedAt   timeutil.Time `json:"createdAt"   cbor:"createdAt"   example:"2024-01-15T10:30:00.000Z"`
	Description string        `json:"description" cbor:"description" example:"A compact electronic widget for everyday use"`
}

// Money represents an exact monetary amount in minor currency units.
type Money struct {
	AmountMinor int64  `json:"amountMinor" cbor:"amountMinor" example:"2999"`
	Currency    string `json:"currency"    cbor:"currency"    example:"USD"`
}

func dollars(amountMinor int64) Money {
	return Money{AmountMinor: amountMinor, Currency: "USD"}
}

// ListData is the response body containing paginated items.
type ListData struct {
	Items []Item `json:"items" cbor:"items"`
	Total int    `json:"total" cbor:"total" example:"30"`
}

// catalog is the accepted deterministic item collection.
var catalog = []Item{
	{
		ID:          "item-001",
		Name:        "Alpha Widget",
		Category:    "electronics",
		Price:       dollars(2999),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
		Description: "A versatile electronic widget for everyday use",
	},
	{
		ID:          "item-002",
		Name:        "Beta Gadget",
		Category:    "electronics",
		Price:       dollars(4999),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 16, 11, 0, 0, 0, time.UTC)),
		Description: "Advanced gadget with smart features",
	},
	{
		ID:          "item-003",
		Name:        "Gamma Tool",
		Category:    "tools",
		Price:       dollars(1550),
		InStock:     false,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 17, 9, 15, 0, 0, time.UTC)),
		Description: "Precision tool for professional work",
	},
	{
		ID:          "item-004",
		Name:        "Delta Component",
		Category:    "electronics",
		Price:       dollars(899),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 18, 14, 45, 0, 0, time.UTC)),
		Description: "Essential component for electronics projects",
	},
	{
		ID:          "item-005",
		Name:        "Epsilon Sensor",
		Category:    "electronics",
		Price:       dollars(3499),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 19, 8, 0, 0, 0, time.UTC)),
		Description: "High-precision environmental sensor",
	},
	{
		ID:          "item-006",
		Name:        "Zeta Cable",
		Category:    "accessories",
		Price:       dollars(1299),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 20, 16, 30, 0, 0, time.UTC)),
		Description: "Premium quality data cable",
	},
	{
		ID:          "item-007",
		Name:        "Eta Adapter",
		Category:    "accessories",
		Price:       dollars(999),
		InStock:     false,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC)),
		Description: "Universal power adapter",
	},
	{
		ID:          "item-008",
		Name:        "Theta Board",
		Category:    "electronics",
		Price:       dollars(8999),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 22, 11, 30, 0, 0, time.UTC)),
		Description: "Development board for prototyping",
	},
	{
		ID:          "item-009",
		Name:        "Iota Switch",
		Category:    "electronics",
		Price:       dollars(599),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 23, 9, 45, 0, 0, time.UTC)),
		Description: "Tactile push button switch",
	},
	{
		ID:          "item-010",
		Name:        "Kappa Display",
		Category:    "electronics",
		Price:       dollars(4599),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 24, 13, 0, 0, 0, time.UTC)),
		Description: "OLED display module",
	},
	{
		ID:          "item-011",
		Name:        "Lambda Motor",
		Category:    "robotics",
		Price:       dollars(2499),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 25, 8, 30, 0, 0, time.UTC)),
		Description: "DC motor for robotics projects",
	},
	{
		ID:          "item-012",
		Name:        "Mu Servo",
		Category:    "robotics",
		Price:       dollars(1899),
		InStock:     false,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 26, 15, 0, 0, 0, time.UTC)),
		Description: "High-torque servo motor",
	},
	{
		ID:          "item-013",
		Name:        "Nu Battery",
		Category:    "power",
		Price:       dollars(1499),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 27, 10, 15, 0, 0, time.UTC)),
		Description: "Rechargeable lithium battery pack",
	},
	{
		ID:          "item-014",
		Name:        "Xi Charger",
		Category:    "power",
		Price:       dollars(2299),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 28, 11, 45, 0, 0, time.UTC)),
		Description: "Smart battery charger",
	},
	{
		ID:          "item-015",
		Name:        "Omicron Relay",
		Category:    "electronics",
		Price:       dollars(799),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 29, 9, 0, 0, 0, time.UTC)),
		Description: "5V relay module",
	},
	{
		ID:          "item-016",
		Name:        "Pi Controller",
		Category:    "electronics",
		Price:       dollars(5599),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 1, 30, 14, 30, 0, 0, time.UTC)),
		Description: "Microcontroller board",
	},
	{
		ID:          "item-017",
		Name:        "Rho Resistor Kit",
		Category:    "components",
		Price:       dollars(1199),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 1, 8, 0, 0, 0, time.UTC)),
		Description: "Assorted resistor pack",
	},
	{
		ID:          "item-018",
		Name:        "Sigma Capacitor Set",
		Category:    "components",
		Price:       dollars(1399),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 2, 10, 30, 0, 0, time.UTC)),
		Description: "Electrolytic capacitor assortment",
	},
	{
		ID:          "item-019",
		Name:        "Tau LED Pack",
		Category:    "components",
		Price:       dollars(699),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 3, 11, 0, 0, 0, time.UTC)),
		Description: "Multi-color LED assortment",
	},
	{
		ID:          "item-020",
		Name:        "Upsilon Wire Set",
		Category:    "accessories",
		Price:       dollars(899),
		InStock:     false,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 4, 9, 15, 0, 0, time.UTC)),
		Description: "Jumper wire kit",
	},
	{
		ID:          "item-021",
		Name:        "Phi Breadboard",
		Category:    "tools",
		Price:       dollars(499),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 5, 13, 45, 0, 0, time.UTC)),
		Description: "Solderless breadboard",
	},
	{
		ID:          "item-022",
		Name:        "Chi Soldering Iron",
		Category:    "tools",
		Price:       dollars(3599),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 6, 10, 0, 0, 0, time.UTC)),
		Description: "Temperature-controlled soldering station",
	},
	{
		ID:          "item-023",
		Name:        "Psi Multimeter",
		Category:    "tools",
		Price:       dollars(4299),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 7, 11, 30, 0, 0, time.UTC)),
		Description: "Digital multimeter with auto-ranging",
	},
	{
		ID:          "item-024",
		Name:        "Omega Oscilloscope",
		Category:    "tools",
		Price:       dollars(29999),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 8, 14, 0, 0, 0, time.UTC)),
		Description: "Portable digital oscilloscope",
	},
	{
		ID:          "item-025",
		Name:        "Alpha Pro Widget",
		Category:    "electronics",
		Price:       dollars(5999),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 9, 8, 30, 0, 0, time.UTC)),
		Description: "Professional-grade widget with extended features",
	},
	{
		ID:          "item-026",
		Name:        "Beta Max Gadget",
		Category:    "electronics",
		Price:       dollars(7999),
		InStock:     false,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 10, 9, 0, 0, 0, time.UTC)),
		Description: "Maximum performance gadget",
	},
	{
		ID:          "item-027",
		Name:        "Gamma Plus Tool",
		Category:    "tools",
		Price:       dollars(2599),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 11, 10, 15, 0, 0, time.UTC)),
		Description: "Enhanced precision tool",
	},
	{
		ID:          "item-028",
		Name:        "Delta Ultra Component",
		Category:    "electronics",
		Price:       dollars(1699),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 12, 11, 45, 0, 0, time.UTC)),
		Description: "Ultra-reliable component",
	},
	{
		ID:          "item-029",
		Name:        "Epsilon HD Sensor",
		Category:    "electronics",
		Price:       dollars(5499),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 13, 13, 0, 0, 0, time.UTC)),
		Description: "High-definition sensor array",
	},
	{
		ID:          "item-030",
		Name:        "Zeta Premium Cable",
		Category:    "accessories",
		Price:       dollars(1999),
		InStock:     true,
		CreatedAt:   timeutil.NewTime(time.Date(2024, 2, 14, 15, 30, 0, 0, time.UTC)),
		Description: "Gold-plated premium cable",
	},
}

// Catalog returns a copy of the accepted deterministic item collection.
// The OpenAPI normalizer uses the same values to constrain the read-only response schema.
func Catalog() []Item {
	return slices.Clone(catalog)
}

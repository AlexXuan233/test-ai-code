package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"fraud-scorer/internal/models"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	var txs []models.ScoreRequest
	baseTime := time.Now().Add(-72 * time.Hour)

	currencies := []string{"USD", "EUR", "AED"}
	countries := []string{"US", "GB", "DE", "FR", "AE", "SA", "JP", "SG", "BR", "RU", "IN", "CN"}
	emails := generateEmails(40)
	cards := generateCards(50)
	addresses := generateAddresses(30)
	ips := generateIPs(60)

	// Helper to pick random
	randCurr := func() string { return currencies[rand.Intn(len(currencies))] }
	randCountry := func() string { return countries[rand.Intn(len(countries))] }
	randEmail := func() string { return emails[rand.Intn(len(emails))] }
	randCard := func() string { return cards[rand.Intn(len(cards))] }
	randAddr := func() string { return addresses[rand.Intn(len(addresses))] }
	randIP := func() string { return ips[rand.Intn(len(ips))] }

	// 1. Normal legitimate transactions (~220, 70-80%)
	for i := 0; i < 220; i++ {
		email := randEmail()
		country := randCountry()
		txs = append(txs, models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-%d", i+1),
			Timestamp:       baseTime.Add(time.Duration(rand.Intn(72)) * time.Hour).Add(time.Duration(rand.Intn(60)) * time.Minute).Format(time.RFC3339),
			CardNumber:      randCard(),
			Amount:          float64(rand.Intn(13000) + 2000),
			Currency:        randCurr(),
			CustomerEmail:   email,
			ShippingAddress: randAddr(),
			ShippingCountry: country,
			BillingCountry:  country,
			IPAddress:       randIP(),
			CustomerID:      fmt.Sprintf("CUST-%s", hashStr(email)),
			IsFirstTime:     false,
			CardBIN:         fmt.Sprintf("%d", 400000+rand.Intn(99999)),
			DeviceID:        fmt.Sprintf("DEV-%d", rand.Intn(100)),
		})
	}

	// 2. Velocity attack: 8 transactions from same card in 8 minutes
	attackCard := "4532123456789012"
	attackTime := baseTime.Add(48 * time.Hour)
	for i := 0; i < 8; i++ {
		txs = append(txs, models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-VEL-%d", i+1),
			Timestamp:       attackTime.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			CardNumber:      attackCard,
			Amount:          float64(rand.Intn(5000) + 2000),
			Currency:        "USD",
			CustomerEmail:   fmt.Sprintf("attacker%d@tempmail.com", i),
			ShippingAddress: "123 Scam St, Dubai",
			ShippingCountry: "AE",
			BillingCountry:  "AE",
			IPAddress:       "203.0.113.50",
			CustomerID:      fmt.Sprintf("CUST-ATTACK-%d", i),
			IsFirstTime:     true,
			CardBIN:         "453212",
			DeviceID:        "DEV-ATTACK",
		})
	}

	// 3. Shipping address scam: 6 orders to same address from different cards
	scamAddress := "45 Luxury Ave, Dubai Marina, UAE"
	scamTime := baseTime.Add(50 * time.Hour)
	scamIPs := []string{"198.51.100.10", "198.51.100.11", "198.51.100.12", "198.51.100.13", "198.51.100.14", "198.51.100.15"}
	for i := 0; i < 6; i++ {
		txs = append(txs, models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-SHIP-%d", i+1),
			Timestamp:       scamTime.Add(time.Duration(rand.Intn(60)) * time.Minute).Format(time.RFC3339),
			CardNumber:      generateCards(1)[0],
			Amount:          float64(rand.Intn(8000) + 3000),
			Currency:        "AED",
			CustomerEmail:   fmt.Sprintf("buyer%d@anonymous.com", i),
			ShippingAddress: scamAddress,
			ShippingCountry: "AE",
			BillingCountry:  "AE",
			IPAddress:       scamIPs[i],
			CustomerID:      fmt.Sprintf("CUST-SCAM-%d", i),
			IsFirstTime:     true,
			CardBIN:         fmt.Sprintf("%d", 510000+rand.Intn(99999)),
			DeviceID:        fmt.Sprintf("DEV-SCAM-%d", i),
		})
	}

	// 4. High-value first-time purchases (3 transactions >$20k)
	for i := 0; i < 3; i++ {
		txs = append(txs, models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-HIGH-%d", i+1),
			Timestamp:       baseTime.Add(60*time.Hour + time.Duration(i)*time.Hour).Format(time.RFC3339),
			CardNumber:      generateCards(1)[0],
			Amount:          float64(rand.Intn(15000) + 20000),
			Currency:        "EUR",
			CustomerEmail:   fmt.Sprintf("newrich%d@mail.com", i),
			ShippingAddress: randAddr(),
			ShippingCountry: "FR",
			BillingCountry:  "FR",
			IPAddress:       randIP(),
			CustomerID:      fmt.Sprintf("CUST-NEW-%d", i),
			IsFirstTime:     true,
			CardBIN:         "411111",
			DeviceID:        fmt.Sprintf("DEV-NEW-%d", i),
		})
	}

	// 5. Geographic mismatches: card issued Brazil, shipping Russia, etc.
	mismatches := []struct{ billing, shipping string }{
		{"BR", "RU"},
		{"US", "CN"},
		{"DE", "IN"},
		{"JP", "AE"},
	}
	for i, mm := range mismatches {
		txs = append(txs, models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-GEO-%d", i+1),
			Timestamp:       baseTime.Add(55*time.Hour + time.Duration(i)*time.Minute*30).Format(time.RFC3339),
			CardNumber:      generateCards(1)[0],
			Amount:          float64(rand.Intn(8000) + 4000),
			Currency:        "USD",
			CustomerEmail:   fmt.Sprintf("global%d@shopper.com", i),
			ShippingAddress: randAddr(),
			ShippingCountry: mm.shipping,
			BillingCountry:  mm.billing,
			IPAddress:       randIP(),
			CustomerID:      fmt.Sprintf("CUST-GLOBAL-%d", i),
			IsFirstTime:     false,
			CardBIN:         fmt.Sprintf("%d", 400000+rand.Intn(99999)),
			DeviceID:        fmt.Sprintf("DEV-GLOBAL-%d", i),
		})
	}

	// 6. Known proxy/VPN IPs
	proxyIPs := []string{"10.0.0.50", "10.0.0.51", "10.0.0.52"}
	for i, pip := range proxyIPs {
		txs = append(txs, models.ScoreRequest{
			TransactionID:   fmt.Sprintf("TX-PROXY-%d", i+1),
			Timestamp:       baseTime.Add(65*time.Hour + time.Duration(i)*time.Minute*15).Format(time.RFC3339),
			CardNumber:      generateCards(1)[0],
			Amount:          float64(rand.Intn(5000) + 2000),
			Currency:        "AED",
			CustomerEmail:   fmt.Sprintf("proxy%d@dark.net", i),
			ShippingAddress: randAddr(),
			ShippingCountry: "AE",
			BillingCountry:  "AE",
			IPAddress:       pip,
			CustomerID:      fmt.Sprintf("CUST-PROXY-%d", i),
			IsFirstTime:     true,
			CardBIN:         "400000",
			DeviceID:        fmt.Sprintf("DEV-PROXY-%d", i),
		})
	}

	// 7. Repeat legitimate customers with consistent patterns
	loyalCustomers := []struct {
		email, country, card string
	}{
		{"sarah.alrashed@email.ae", "AE", "4242424242424242"},
		{"li.wei@shop.cn", "CN", "5555555555554444"},
		{"jean.dupont@mail.fr", "FR", "378282246310005"},
	}
	for i, lc := range loyalCustomers {
		for j := 0; j < 5; j++ {
			txs = append(txs, models.ScoreRequest{
				TransactionID:   fmt.Sprintf("TX-LOYAL-%d-%d", i+1, j+1),
				Timestamp:       baseTime.Add(time.Duration(j*12) * time.Hour).Format(time.RFC3339),
				CardNumber:      lc.card,
				Amount:          float64(rand.Intn(6000) + 3000),
				Currency:        []string{"AED", "CNY", "EUR"}[i],
				CustomerEmail:   lc.email,
				ShippingAddress: generateAddresses(1)[0],
				ShippingCountry: lc.country,
				BillingCountry:  lc.country,
				IPAddress:       fmt.Sprintf("192.168.%d.%d", i+1, j+10),
				CustomerID:      fmt.Sprintf("CUST-LOYAL-%d", i+1),
				IsFirstTime:     false,
				CardBIN:         lc.card[:6],
				DeviceID:        fmt.Sprintf("DEV-LOYAL-%d", i+1),
			})
		}
	}

	data, err := json.MarshalIndent(txs, "", "  ")
	if err != nil {
		fmt.Println("marshal error:", err)
		os.Exit(1)
	}

	if err := os.WriteFile("testdata/transactions.json", data, 0644); err != nil {
		fmt.Println("write error:", err)
		os.Exit(1)
	}

	fmt.Printf("generated %d test transactions -> testdata/transactions.json\n", len(txs))
}

func generateEmails(n int) []string {
	bases := []string{"sarah", "mohammed", "li", "jean", "olga", "raj", "kim", "david", "anna", "carlos", "emma", "noah", "yuki", "ahmed", "maria"}
	domains := []string{"gmail.com", "outlook.com", "protonmail.com", "email.ae", "shop.cn", "mail.fr", "yahoo.co.jp"}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		b := bases[rand.Intn(len(bases))]
		d := domains[rand.Intn(len(domains))]
		out = append(out, fmt.Sprintf("%s%d@%s", b, rand.Intn(9999), d))
	}
	return out
}

func generateCards(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%d", 4000000000000000+rand.Intn(999999999999999)))
	}
	return out
}

func generateAddresses(n int) []string {
	cities := []string{"Dubai", "Abu Dhabi", "London", "Paris", "New York", "Singapore", "Tokyo", "Berlin", "Mumbai", "Sydney"}
	streets := []string{"Main St", "King Rd", "Queen Ave", "Ocean Dr", "Park Ln", "River Rd", "Hill St", "Garden Ave"}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%d %s, %s", rand.Intn(999), streets[rand.Intn(len(streets))], cities[rand.Intn(len(cities))]))
	}
	return out
}

func generateIPs(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("%d.%d.%d.%d", rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(256)))
	}
	return out
}

func hashStr(s string) string {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("%d", h%100000)
}

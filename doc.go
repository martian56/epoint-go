// Package epoint is a Go client for the epoint.az payment gateway.
//
// It covers all 30 documented endpoints: payments, saved cards, refunds and
// payouts, split payments, pre-authorisation, installments, wallets, Apple Pay
// and Google Pay, invoices, and B2B transfers.
//
// Create a client and start a payment:
//
//	client := epoint.New("i000000001", "your-private-key")
//	payment, err := client.CreatePayment(ctx, 30.75, "order-1", nil)
//	if err != nil {
//		return err
//	}
//	// send the customer to payment.RedirectURL()
//
// Point BaseURL at the local sandbox to build and test without a merchant
// account: https://github.com/martian56/epoint-sandbox
package epoint

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"eth-mempool-monitor/internal/mempool"

	"github.com/rivo/tview"
)

func main() {
	// Create a new context and cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up buffered channels for transaction updates, decoded transaction details, and TPS
	txChan := make(chan string, 10)
	txDetailsChan := make(chan string, 10)
	tpsChan := make(chan uint64, 10)

	// Setup signal handling to exit gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Initialize application
	app := tview.NewApplication()

	// Initialize TextViews for transaction and details
	tpsView := tview.NewTextView().
		SetText("Transactions Per Second (TPS): 0").
		SetDynamicColors(true).
		SetScrollable(false)

	txView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	txDetailsView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	// // Capture input to handle scrolling
	// txDetailsView.SetInputCapture(func(event *tview.EventKey) *tview.EventKey {
	// 	switch event.Key() {
	// 	case tview.KeyUp: // Scroll up
	// 		txDetailsView.ScrollUp()
	// 	case tview.KeyDown: // Scroll down
	// 		txDetailsView.ScrollDown()
	// 	case tview.KeyPgUp: // Page up
	// 		txDetailsView.ScrollPageUp()
	// 	case tview.KeyPgDn: // Page down
	// 		txDetailsView.ScrollPageDown()
	// 	}
	// 	return event
	// })

	// txView.SetInputCapture(func(event *tview.EventKey) *tview.EventKey {
	// 	switch event.Key() {
	// 	case tview.KeyUp: // Scroll up
	// 		txView.ScrollUp()
	// 	case tview.KeyDown: // Scroll down
	// 		txView.ScrollDown()
	// 	case tview.KeyPgUp: // Page up
	// 		txView.ScrollPageUp()
	// 	case tview.KeyPgDn: // Page down
	// 		txView.ScrollPageDown()
	// 	}
	// 	return event
	// })

	// Create a grid layout
	grid := tview.NewGrid().
		SetRows(3, 0).    // Two rows: 3 height for the TPS, and the rest for the transaction views
		SetColumns(0, 0). // Two columns: equally split for txView and txDetailsView
		SetBorders(true).
		AddItem(tpsView, 0, 0, 1, 2, 0, 0, false).     // TPS view at the top, spanning two columns
		AddItem(txView, 1, 0, 1, 1, 0, 0, true).       // Transactions list on the left
		AddItem(txDetailsView, 1, 1, 1, 1, 0, 0, true) // Transaction details on the right

	// Improved goroutine for handling transaction data
	go func() {
		for {
			select {
			case <-sigCh:
				cancel() // Signal to cancel the context and stop all goroutines
				return
			case tps := <-tpsChan:
				app.QueueUpdateDraw(func() {
					tpsView.SetText(fmt.Sprintf("Transactions Per Second (TPS): %d", tps))
				})
			case tx := <-txChan:
				app.QueueUpdateDraw(func() {
					currentTxText := txView.GetText(true)
					newTxText := currentTxText + tx + "\n" // Append new transaction details
					txView.SetText(newTxText)
					txView.ScrollToEnd() // Scroll to end after updating
				})
			case txDetails := <-txDetailsChan:
				app.QueueUpdateDraw(func() {
					currentDetailsText := txDetailsView.GetText(true)
					newDetailsText := currentDetailsText + txDetails + "\n" // Append new decoded transaction details
					txDetailsView.SetText(newDetailsText)
					txDetailsView.ScrollToEnd() // Scroll to end after updating
				})
			}
		}
	}()

	// Start the mempool monitoring
	go mempool.MonitorMempool(ctx, tpsChan, txChan, txDetailsChan)

	// Run the application
	if err := app.SetRoot(grid, true).Run(); err != nil {
		log.Fatalf("failed to run application: %v", err)
	}
}

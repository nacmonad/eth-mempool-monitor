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

	// Set up buffered channels for transaction updates, decoded transaction details, TPS, and logs
	txChan := make(chan string, 10)
	txDetailsChan := make(chan string, 10)
	tpsChan := make(chan uint64, 10)
	logChan := make(chan string, 10) // Channel for log messages

	// Setup signal handling to exit gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Initialize application
	app := tview.NewApplication()

	// Initialize TextViews for transaction, details, and logs
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

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetRegions(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	// Create a grid layout with an additional row for logs
	grid := tview.NewGrid().
		SetRows(3, 0, 5). // Three rows: TPS, transactions, and logs
		SetColumns(0, 0). // Two columns: transactions and details
		SetBorders(true).
		AddItem(tpsView, 0, 0, 1, 2, 0, 0, false).      // TPS view at the top, spanning two columns
		AddItem(txView, 1, 0, 1, 1, 0, 0, true).        // Transactions list on the left
		AddItem(txDetailsView, 1, 1, 1, 1, 0, 0, true). // Transaction details on the right
		AddItem(logView, 2, 0, 1, 2, 0, 0, false)       // Log view at the bottom, spanning two columns

	// Goroutine for handling transaction data and logs
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
			case logMsg := <-logChan:
				app.QueueUpdateDraw(func() {
					currentLogText := logView.GetText(true)
					newLogText := currentLogText + logMsg + "\n" // Append new log messages
					logView.SetText(newLogText)
					logView.ScrollToEnd() // Scroll to end after updating
				})
			}
		}
	}()

	// Redirect standard log output to the log channel
	log.SetOutput(logWriter(logChan))

	// Start the mempool monitoring
	go mempool.MonitorMempool(ctx, tpsChan, txChan, txDetailsChan)

	// Run the application
	if err := app.SetRoot(grid, true).Run(); err != nil {
		log.Fatalf("failed to run application: %v", err)
	}
}

// logWriter is a custom log writer that sends log messages to the log channel
func logWriter(logChan chan<- string) *writerAdapter {
	return &writerAdapter{logChan: logChan}
}

type writerAdapter struct {
	logChan chan<- string
}

func (w *writerAdapter) Write(p []byte) (n int, err error) {
	w.logChan <- string(p)
	return len(p), nil
}

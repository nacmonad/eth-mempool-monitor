package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"eth-mempool-monitor/internal/mempool"

	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

func main() {
	// Initialize Termui
	if err := termui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer termui.Close()

	// Create a new context and cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up channels for transaction updates and TPS
	txChan := make(chan string)
	tpsChan := make(chan uint64)

	// Setup signal handling to exit gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Initialize dashboard elements
	tpsWidget := widgets.NewParagraph()
	tpsWidget.Title = "Transactions Per Second (TPS)"
	tpsWidget.Text = "0"
	tpsWidget.SetRect(0, 0, 30, 3)

	txList := widgets.NewList()
	txList.Title = "Recent Transactions"
	txList.Rows = []string{}
	txList.WrapText = false
	txList.SetRect(0, 3, 100, 30)

	// Render loop
	go func() {
		for {
			select {
			case <-sigCh:
				cancel() // Signal to cancel the context and stop all goroutines
				return
			case tps := <-tpsChan:
				tpsWidget.Text = fmt.Sprintf("%d", tps)
			case tx := <-txChan:
				txList.Rows = append([]string{tx}, txList.Rows...)
				if len(txList.Rows) > 30 {
					txList.Rows = txList.Rows[:30]
				}
			}
			termui.Render(tpsWidget, txList)
		}
	}()

	// Start the mempool monitoring
	go mempool.MonitorMempool(ctx, txChan, tpsChan)

	// Wait for the context to be canceled (either by interrupt or by cancel)
	<-ctx.Done()

	fmt.Println("Exiting application...")
}

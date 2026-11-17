package ops

import "sync/atomic"

// paidReads counts the reads of the paid-ground file, through the hook the ledger offers (the shipped build keeps no counter).
var paidReads atomic.Int64

func init() { paidReadHook = func() { paidReads.Add(1) } }

// recorded is the highest ground recorded for the character on the house in the term, under any faction, or 0. It reads the file; the editor's own code asks a snapshot.
func (l *PaidLedger) recorded(player, term int64, house string) float64 {
	es, _ := l.snapshot()
	return highestPaid(es, player, term, house)
}

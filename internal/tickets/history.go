package tickets

import (
	"crypto/rand"
	"fmt"
	"sort"
	"time"
)

func newOperationID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate operation id: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func buildStatusHistory(operationID string, before, after Status, actor Actor, changedAt time.Time) []HistoryItem {
	beforeValue, afterValue := string(before), string(after)
	return []HistoryItem{{
		OperationID: operationID,
		Actor:       actor,
		ChangedAt:   changedAt.UTC(),
		Changes:     []HistoryChange{{Field: HistoryFieldStatus, Before: &beforeValue, After: &afterValue}},
	}}
}

func toHistoryRecords(owner string, ticketID uint64, items []HistoryItem) []historyRecord {
	var records []historyRecord
	for _, item := range items {
		for _, change := range item.Changes {
			records = append(records, historyRecord{
				OwnerUserID: owner,
				TicketID:    ticketID,
				OperationID: item.OperationID,
				Field:       string(change.Field),
				BeforeValue: change.Before,
				AfterValue:  change.After,
				ChangedBy:   item.Actor,
				ChangedAt:   item.ChangedAt.UTC(),
			})
		}
	}
	return records
}

// groupHistoryRecords は差分行を operation_id ごとにまとめ、新しい順に並べる。
func groupHistoryRecords(records []historyRecord) []HistoryItem {
	sorted := append([]historyRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if !left.ChangedAt.Equal(right.ChangedAt) {
			return left.ChangedAt.After(right.ChangedAt)
		}
		if left.OperationID != right.OperationID {
			return left.OperationID < right.OperationID
		}
		return left.ID < right.ID
	})
	items := make([]HistoryItem, 0, len(sorted))
	indexByOperation := make(map[string]int, len(sorted))
	for _, record := range sorted {
		change := HistoryChange{Field: HistoryField(record.Field), Before: record.BeforeValue, After: record.AfterValue}
		index, ok := indexByOperation[record.OperationID]
		if !ok {
			indexByOperation[record.OperationID] = len(items)
			items = append(items, HistoryItem{
				OperationID: record.OperationID,
				Actor:       record.ChangedBy,
				ChangedAt:   record.ChangedAt.UTC(),
				Changes:     []HistoryChange{change},
			})
			continue
		}
		items[index].Changes = append(items[index].Changes, change)
	}
	return items
}

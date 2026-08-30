package events

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetEventCache clears the package-level dedup cache so each subtest starts
// from a known state. lastEventCache and its mutex are shared across the bus,
// so isolation must be explicit.
func resetEventCache(t *testing.T) {
	t.Helper()
	lastEventCacheMutex.Lock()
	lastEventCache = make(map[string]NodeEvent)
	lastEventCacheMutex.Unlock()
}

func TestNodeEventBusShouldFilterEvent(t *testing.T) {
	t.Run("heartbeat filtered when there are no subscribers", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		assert.True(t, bus.shouldFilterEvent(NodeEvent{Type: NodeHeartbeat, NodeID: "n1"}))
	})

	t.Run("heartbeat not filtered when a subscriber exists", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		_ = bus.Subscribe("sub-hb")
		defer bus.Unsubscribe("sub-hb")
		assert.False(t, bus.shouldFilterEvent(NodeEvent{Type: NodeHeartbeat, NodeID: "n1"}))
	})

	t.Run("non-status event is never treated as a duplicate", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		ev := NodeEvent{Type: NodeRegistered, NodeID: "n1", Timestamp: time.Now()}
		assert.False(t, bus.shouldFilterEvent(ev))
		assert.False(t, bus.shouldFilterEvent(ev))
	})
}

func TestNodeEventBusIsDuplicateStatusEvent(t *testing.T) {
	t.Run("first status event is not a duplicate and is cached", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		ev := NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Status: "active", Timestamp: time.Now()}
		assert.False(t, bus.isDuplicateStatusEvent(ev))
	})

	t.Run("identical status event within 1s is a duplicate", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		now := time.Now()
		first := NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Status: "active", Timestamp: now}
		require.False(t, bus.isDuplicateStatusEvent(first))
		// Same status, still within the 1s window.
		second := NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Status: "active", Timestamp: now}
		assert.True(t, bus.isDuplicateStatusEvent(second))
	})

	t.Run("changed status within 1s is not a duplicate", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		now := time.Now()
		require.False(t, bus.isDuplicateStatusEvent(NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Status: "active", Timestamp: now}))
		// Different Status value: not a duplicate even inside the window.
		assert.False(t, bus.isDuplicateStatusEvent(NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Status: "inactive", Timestamp: now}))
	})

	t.Run("changed new/old status on unified event is not a duplicate", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		now := time.Now()
		first := NodeEvent{Type: NodeUnifiedStatusChanged, NodeID: "n1", Status: "s", OldStatus: "a", NewStatus: "b", Timestamp: now}
		require.False(t, bus.isDuplicateStatusEvent(first))
		// Same Status but different NewStatus transition.
		second := NodeEvent{Type: NodeUnifiedStatusChanged, NodeID: "n1", Status: "s", OldStatus: "a", NewStatus: "c", Timestamp: now}
		assert.False(t, bus.isDuplicateStatusEvent(second))
	})

	t.Run("non-comparable status event within 1s is a duplicate", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		now := time.Now()
		// NodeOnline is a status event type but not one of the compared kinds,
		// so a second one within the window is a flat duplicate.
		first := NodeEvent{Type: NodeOnline, NodeID: "n1", Timestamp: now}
		require.False(t, bus.isDuplicateStatusEvent(first))
		assert.True(t, bus.isDuplicateStatusEvent(NodeEvent{Type: NodeOnline, NodeID: "n1", Timestamp: now}))
	})

	t.Run("stale cached event outside 1s window is not a duplicate", func(t *testing.T) {
		resetEventCache(t)
		bus := NewNodeEventBus()
		old := NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Status: "active", Timestamp: time.Now().Add(-2 * time.Second)}
		lastEventCacheMutex.Lock()
		lastEventCache["node_status_updated:n1"] = old
		lastEventCacheMutex.Unlock()

		fresh := NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Status: "active", Timestamp: time.Now()}
		assert.False(t, bus.isDuplicateStatusEvent(fresh))
	})
}

func TestNodeEventBusCompareStatusEventData(t *testing.T) {
	bus := NewNodeEventBus()

	t.Run("different status is not equal", func(t *testing.T) {
		assert.False(t, bus.compareStatusEventData(
			NodeEvent{Status: "a"},
			NodeEvent{Status: "b"},
		))
	})

	t.Run("same status non-unified is equal", func(t *testing.T) {
		assert.True(t, bus.compareStatusEventData(
			NodeEvent{Type: NodeStatusUpdated, Status: "a"},
			NodeEvent{Type: NodeStatusUpdated, Status: "a"},
		))
	})

	t.Run("unified event compares old/new status", func(t *testing.T) {
		last := NodeEvent{Type: NodeUnifiedStatusChanged, Status: "a", OldStatus: "x", NewStatus: "y"}
		assert.True(t, bus.compareStatusEventData(last, NodeEvent{Type: NodeUnifiedStatusChanged, Status: "a", OldStatus: "x", NewStatus: "y"}))
		assert.False(t, bus.compareStatusEventData(last, NodeEvent{Type: NodeUnifiedStatusChanged, Status: "a", OldStatus: "x", NewStatus: "z"}))
	})
}

func TestNodeEventBusCleanupEventCache(t *testing.T) {
	resetEventCache(t)
	bus := NewNodeEventBus()

	lastEventCacheMutex.Lock()
	lastEventCache["stale:n1"] = NodeEvent{Type: NodeStatusUpdated, NodeID: "n1", Timestamp: time.Now().Add(-10 * time.Minute)}
	lastEventCache["fresh:n2"] = NodeEvent{Type: NodeStatusUpdated, NodeID: "n2", Timestamp: time.Now()}
	lastEventCacheMutex.Unlock()

	bus.cleanupEventCache()

	lastEventCacheMutex.RLock()
	_, staleExists := lastEventCache["stale:n1"]
	_, freshExists := lastEventCache["fresh:n2"]
	lastEventCacheMutex.RUnlock()

	assert.False(t, staleExists, "entries older than 5 minutes should be dropped")
	assert.True(t, freshExists, "recent entries should be retained")
}

func TestPublishNodeStatusUpdatedEnhancedExtractsState(t *testing.T) {
	resetEventCache(t)
	subID := "test-enhanced-state"
	ch := GlobalNodeEventBus.Subscribe(subID)
	defer GlobalNodeEventBus.Unsubscribe(subID)

	// newStatus is a map carrying a "state" field: the legacy NodeStatusUpdated
	// event should carry that extracted state string. Both the unified and the
	// legacy event are published, so drain until the legacy one arrives.
	PublishNodeStatusUpdatedEnhanced("node-state", nil, map[string]interface{}{"state": "ready"}, "src", "reason")

	var legacy *NodeEvent
	deadline := time.After(5 * time.Second)
	for legacy == nil {
		select {
		case ev := <-ch:
			if ev.Type == NodeStatusUpdated {
				e := ev
				legacy = &e
			}
		case <-deadline:
			t.Fatal("timed out waiting for legacy NodeStatusUpdated event")
		}
	}
	assert.Equal(t, "ready", legacy.Status)
}

func TestStartNodeHeartbeatEmitsOnlyWithSubscribers(t *testing.T) {
	resetEventCache(t)
	// No subscribers: PublishNodeHeartbeat filtered out by shouldFilterEvent,
	// so nothing is delivered. With a subscriber, the ticker delivers one.
	StartNodeHeartbeat(10 * time.Millisecond)

	subID := "test-node-hb-start"
	ch := GlobalNodeEventBus.Subscribe(subID)
	defer GlobalNodeEventBus.Unsubscribe(subID)

	select {
	case ev := <-ch:
		assert.Equal(t, NodeHeartbeat, ev.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("expected a heartbeat once a subscriber was present")
	}
}

// Guard against a data race on the shared cache when helpers run concurrently.
func TestIsDuplicateStatusEventConcurrent(t *testing.T) {
	resetEventCache(t)
	bus := NewNodeEventBus()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = bus.isDuplicateStatusEvent(NodeEvent{Type: NodeStatusUpdated, NodeID: "race", Status: "active", Timestamp: time.Now()})
		}(i)
	}
	wg.Wait()
}

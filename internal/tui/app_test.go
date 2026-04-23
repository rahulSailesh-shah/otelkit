package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func keyShiftTab() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift})
}

func TestAppDelegatesTabAndEscToLogsFilterMode(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	m.activeTab = tabLogs
	m.logsView = viewLogList
	m.logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyRunes("f"))
	am := model.(appModel)
	if !am.logs.filterMode {
		t.Fatal("expected filter mode after f")
	}
	if am.activeTab != tabLogs {
		t.Fatalf("activeTab = %v want logs", am.activeTab)
	}

	model, _ = model.Update(keyCode(tea.KeyTab))
	am = model.(appModel)
	if am.activeTab != tabLogs {
		t.Fatalf("tab should not switch app tab in filter mode, got activeTab=%v", am.activeTab)
	}
	if am.logs.activeFilterField != logsFilterFieldService {
		t.Fatalf("tab should cycle filter field, got activeFilterField=%v want service", am.logs.activeFilterField)
	}

	model, _ = model.Update(keyCode(tea.KeyEsc))
	am = model.(appModel)
	if am.logs.filterMode {
		t.Fatal("esc should exit filter mode when routed through app")
	}
}

func TestAppEnterDoesNotOpenLogDetailInLogsFilterMode(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	m.activeTab = tabLogs
	m.logsView = viewLogList
	m.logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyRunes("f"))
	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	am := model.(appModel)
	if am.logsView != viewLogList {
		t.Fatalf("enter in filter mode should not open detail, got logsView=%v", am.logsView)
	}
	if !am.logs.filterMode {
		t.Fatal("enter on severity field should keep filter mode active")
	}
}

func TestAppShiftTabDoesNotSwitchAppTabInLogsFilterMode(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	m.activeTab = tabLogs
	m.logsView = viewLogList
	m.logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyRunes("f"))
	// With severity (field 0) active, shift+tab cycles backward to body (field 2).
	model, _ = model.Update(keyShiftTab())
	am := model.(appModel)
	if am.activeTab != tabLogs {
		t.Fatalf("shift+tab should not switch app tab in filter mode, got activeTab=%v", am.activeTab)
	}
	if am.logs.activeFilterField != logsFilterFieldBody {
		t.Fatalf("shift+tab should cycle filter field backward, got activeFilterField=%v want body", am.logs.activeFilterField)
	}
}

func TestAppTabStillSwitchesTabsWhenLogsFilterInactive(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	m.activeTab = tabLogs
	m.logsView = viewLogList
	m.logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyCode(tea.KeyTab))
	am := model.(appModel)
	if am.activeTab == tabLogs {
		t.Fatal("expected tab key to leave logs tab when filter mode is off")
	}
}

func TestAppShiftTabStillSwitchesTabsWhenLogsFilterInactive(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	m.activeTab = tabLogs
	m.logsView = viewLogList
	m.logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyShiftTab())
	am := model.(appModel)
	if am.activeTab == tabLogs {
		t.Fatal("expected shift+tab to leave logs tab when filter mode is off")
	}
}

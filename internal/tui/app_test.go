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
	setActiveTabForTest(t, &m, tabLogs)
	logs := mustLogsTabForTest(t, m)
	logs.view = logsViewList
	logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyRunes("f"))
	am := model.(appModel)
	if !mustLogsTabForTest(t, am).filter.Active() {
		t.Fatal("expected filter mode after f")
	}
	if am.ids[am.active] != tabLogs {
		t.Fatalf("active tab = %v want logs", am.ids[am.active])
	}

	model, _ = model.Update(keyCode(tea.KeyTab))
	am = model.(appModel)
	if am.ids[am.active] != tabLogs {
		t.Fatalf("tab should not switch app tab in filter mode, got active=%v", am.ids[am.active])
	}
	if mustLogsTabForTest(t, am).filter.Field() != logsFilterFieldService {
		t.Fatalf("tab should cycle filter field, got activeFilterField=%v want service", mustLogsTabForTest(t, am).filter.Field())
	}

	model, _ = model.Update(keyCode(tea.KeyEsc))
	am = model.(appModel)
	if mustLogsTabForTest(t, am).filter.Active() {
		t.Fatal("esc should exit filter mode when routed through app")
	}
}

func TestAppEnterDoesNotOpenLogDetailInLogsFilterMode(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	setActiveTabForTest(t, &m, tabLogs)
	logs := mustLogsTabForTest(t, m)
	logs.view = logsViewList
	logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyRunes("f"))
	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	am := model.(appModel)
	if mustLogsTabForTest(t, am).view != logsViewList {
		t.Fatalf("enter in filter mode should not open detail, got logsView=%v", mustLogsTabForTest(t, am).view)
	}
	if !mustLogsTabForTest(t, am).filter.Active() {
		t.Fatal("enter on severity field should keep filter mode active")
	}
}

func TestAppShiftTabDoesNotSwitchAppTabInLogsFilterMode(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	setActiveTabForTest(t, &m, tabLogs)
	logs := mustLogsTabForTest(t, m)
	logs.view = logsViewList
	logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyRunes("f"))
	// With severity (field 0) active, shift+tab cycles backward to body (field 2).
	model, _ = model.Update(keyShiftTab())
	am := model.(appModel)
	if am.ids[am.active] != tabLogs {
		t.Fatalf("shift+tab should not switch app tab in filter mode, got active=%v", am.ids[am.active])
	}
	if mustLogsTabForTest(t, am).filter.Field() != logsFilterFieldBody {
		t.Fatalf("shift+tab should cycle filter field backward, got activeFilterField=%v want body", mustLogsTabForTest(t, am).filter.Field())
	}
}

func TestAppTabStillSwitchesTabsWhenLogsFilterInactive(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	setActiveTabForTest(t, &m, tabLogs)
	logs := mustLogsTabForTest(t, m)
	logs.view = logsViewList
	logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyCode(tea.KeyTab))
	am := model.(appModel)
	if am.ids[am.active] == tabLogs {
		t.Fatal("expected tab key to leave logs tab when filter mode is off")
	}
}

func TestAppShiftTabStillSwitchesTabsWhenLogsFilterInactive(t *testing.T) {
	m := newAppModel(context.Background(), Options{})
	setActiveTabForTest(t, &m, tabLogs)
	logs := mustLogsTabForTest(t, m)
	logs.view = logsViewList
	logs.setRows(sampleLogRows())

	var model tea.Model = m
	model, _ = model.Update(keyShiftTab())
	am := model.(appModel)
	if am.ids[am.active] == tabLogs {
		t.Fatal("expected shift+tab to leave logs tab when filter mode is off")
	}
}

func setActiveTabForTest(t *testing.T, m *appModel, id tabID) {
	t.Helper()
	for i, tabID := range m.ids {
		if tabID == id {
			m.active = i
			return
		}
	}
	t.Fatalf("tab %v not found", id)
}

func mustLogsTabForTest(t *testing.T, m appModel) *logsModel {
	t.Helper()
	tab := m.findTab(tabLogs)
	logs, ok := tab.(*logsModel)
	if !ok || logs == nil {
		t.Fatalf("logs tab missing or wrong type: %T", tab)
	}
	return logs
}

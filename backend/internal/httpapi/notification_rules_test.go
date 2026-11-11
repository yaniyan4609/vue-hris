package httpapi

import (
	"testing"
	"time"
)

func TestReminderQuietHoursAndTemplateValidation(t *testing.T) {
	config:=notificationRuleConfig{Enabled:true,OffsetMinutes:-10,RepeatMinutes:30,MaxRepeats:2,QuietStart:"22:00",QuietEnd:"06:00"}
	if !validNotificationRule(config) { t.Fatal("valid reminder rule rejected") }
	start:=time.Date(2026,11,11,16,0,0,0,time.UTC) // 23:00 Jakarta
	got:=afterQuietHours(start,"Asia/Jakarta",config)
	want:=time.Date(2026,11,11,23,0,0,0,time.UTC) // 06:00 Jakarta
	if !got.Equal(want) { t.Fatalf("quiet hours: got %s want %s",got,want) }
	if validNotificationRule(notificationRuleConfig{Enabled:true,RepeatMinutes:0,MaxRepeats:2}) { t.Fatal("repeat without interval accepted") }
	if !validTemplateText("Ada pembaruan di {{company_name}}: {{event_name}}") { t.Fatal("safe variables rejected") }
	if validTemplateText("{{bank_account}}")||validTemplateText("{{company_name}} {{") { t.Fatal("unknown or malformed variable accepted") }
}

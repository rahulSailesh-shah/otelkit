package receiver

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/rahulSailesh-shah/otelkit/internal/export"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
)

type LogsHandler struct {
	db     *sql.DB
	fanout *export.Fanout
	collogspb.UnimplementedLogsServiceServer
}

func NewLogsHandler(db *sql.DB, fanout *export.Fanout) *LogsHandler {
	return &LogsHandler{db: db, fanout: fanout}
}

func (h *LogsHandler) Export(
	ctx context.Context,
	req *collogspb.ExportLogsServiceRequest,
) (*collogspb.ExportLogsServiceResponse, error) {
	params, err := normalizeLogs(req)
	if err != nil {
		return nil, err
	}

	log.Println("[logs] received", len(params), "log records")

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	q := repo.New(tx)
	for _, p := range params {
		if err := q.InsertLogRecord(ctx, p); err != nil {
			log.Printf("[logs] insert log record: %v", err)
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if h.fanout != nil {
		go func(req *collogspb.ExportLogsServiceRequest) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			h.fanout.ExportLogs(bgCtx, req)
		}(req)
	}

	return &collogspb.ExportLogsServiceResponse{}, nil
}

func normalizeLogs(req *collogspb.ExportLogsServiceRequest) ([]repo.InsertLogRecordParams, error) {
	if req == nil {
		return nil, nil
	}

	var params []repo.InsertLogRecordParams

	for _, rl := range req.GetResourceLogs() {
		resourceAttrs, serviceName := extractResourceAttrs(rl.GetResource())
		resourceAttrsJSON := mapToJSONString(resourceAttrs)

		for _, sl := range rl.GetScopeLogs() {
			for _, lr := range sl.GetLogRecords() {
				var body *string
				if b := lr.GetBody(); b != nil {
					s := fmt.Sprintf("%v", anyValueToInterface(b))
					body = &s
				}

				var severity *int64
				if sn := int64(lr.GetSeverityNumber()); sn != 0 {
					severity = &sn
				}

				attrs := mapToJSONString(keyValuesToMap(lr.GetAttributes()))

				params = append(params, repo.InsertLogRecordParams{
					TraceID:       optionalHex(lr.GetTraceId()),
					SpanID:        optionalHex(lr.GetSpanId()),
					ServiceName:   serviceName,
					Severity:      severity,
					SeverityText:  optionalString(lr.GetSeverityText()),
					Body:          body,
					Attributes:    attrs,
					ResourceAttrs: resourceAttrsJSON,
					TimestampNs:   int64(lr.GetTimeUnixNano()),
				})
			}
		}
	}

	return params, nil
}

package kero

import (
	"context"
	plabels "github.com/prometheus/prometheus/model/labels"
	"time"
)

func (k *Kero) Track(metric string, labels MetricLabels, value float64) error {
	app := k.db.Appender(context.Background())
	dbLabels := plabels.FromMap(labels)
	dbLabels = append(dbLabels, plabels.FromStrings(plabels.MetricName, metric)...)
	app.Append(0, dbLabels, time.Now().Unix(), value)
	return app.Commit()
}

func (k *Kero) TrackOne(metric string, labels MetricLabels) error {
	return k.Track(metric, labels, 1)
}

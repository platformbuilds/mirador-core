package utils

import (
    "testing"
    "time"
    "github.com/platformbuilds/mirador-core/internal/models"
)

// Performance Test Cases: micro-benchmarks for utility helpers

func BenchmarkCountSeries(b *testing.B) {
    data := map[string]any{"result": []any{1,2,3,4,5,6,7,8,9,10}}
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _ = CountSeries(data)
    }
}

func BenchmarkCountDataPoints(b *testing.B) {
    series := []any{}
    for i := 0; i < 20; i++ {
        series = append(series, map[string]any{"values": []any{1,2,3,4,5}})
    }
    data := map[string]any{"result": series}
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _ = CountDataPoints(data)
    }
}

func BenchmarkGenerateClientID(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _ = GenerateClientID()
    }
}

func BenchmarkCalculateAvgTimeToFailure(b *testing.B) {
    in := make([]*models.SystemFracture, 0, 100)
    for i := 0; i < 100; i++ {
        in = append(in, &models.SystemFracture{TimeToFracture: time.Second})
    }
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _ = CalculateAvgTimeToFailure(in)
    }
}

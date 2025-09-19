package lucene

import (
    "strings"
    "unicode"
)

// Target identifies which backend query language to generate.
type Target int

const (
    TargetLogsQL Target = iota
    TargetMetricsQL
    TargetTraces
)

// tokenization shared by translators
type tokKind int

const (
    tkEOF tokKind = iota
    tkLParen
    tkRParen
    tkOp
    tkField
    tkBare
)

type token struct{
    k tokKind
    v string
    f string
    val string
    quoted bool
}

// IsLikelyLucene performs a cheap heuristic to identify Lucene-style queries.
// It aims to avoid false-positives with PromQL/LogsQL.
func IsLikelyLucene(s string) bool {
    qs := strings.TrimSpace(s)
    if qs == "" { return false }
    // PromQL usually contains {...} label selectors; LogsQL often uses '='.
    // Prefer Lucene when we see field:value pairs without braces.
    if strings.Contains(qs, "{") && strings.Contains(qs, "}") {
        return false
    }
    // _time:5m, field:"phrase", field:value, parentheses, AND/OR/NOT
    if strings.Contains(qs, ":") {
        return true
    }
    // Bare phrases or tokens alone aren't enough to be confident.
    return false
}

// Translate tries converting a Lucene-like query into the target DSL.
// Returns translated string and ok=false if it could not translate.
func Translate(q string, t Target) (string, bool) {
    switch t {
    case TargetLogsQL:
        return toLogsQL(q)
    case TargetMetricsQL:
        return toMetricsQL(q)
    case TargetTraces:
        // For traces we return empty string here; use TranslateTraces for structured output.
        if _, ok := TranslateTraces(q); ok { return "", true }
        return "", false
    default:
        return "", false
    }
}

// TraceFilters represents extracted filters for Jaeger HTTP API.
type TraceFilters struct {
    Service     string
    Operation   string
    Tags        map[string]string
    MinDuration string
    MaxDuration string
    Since       string // e.g., "15m" → handler converts to Start/End
    StartExpr   string // e.g., "now-15m" or RFC3339
    EndExpr     string // e.g., "now"
}

// TranslateTraces parses a Lucene-like query and extracts Jaeger-compatible filters.
// Supported fields:
// - service:<name>
// - operation:<name>
// - duration:>10ms or duration:<1s (sets MinDuration/MaxDuration)
// - _time:15m (relative window; handler computes absolute times)
// - tag.* or span_attr.* or any other key become tags (k=v; phrases/wildcards use value as-is)
func TranslateTraces(q string) (TraceFilters, bool) {
    s := scanner{src: q}
    out := TraceFilters{Tags: map[string]string{}}
    for {
        t := s.next()
        if t.k == tkEOF { break }
        if t.k != tkField { continue }
        key := strings.ToLower(t.f)
        val := t.val
        if key == "service" {
            out.Service = val
            continue
        }
        if key == "operation" {
            out.Operation = val
            continue
        }
        if key == "duration" {
            v := strings.TrimSpace(val)
            // Support range syntax: [min TO max], {min TO *}, etc.
            if strings.HasPrefix(v, "[") || strings.HasPrefix(v, "{") {
                v = strings.Trim(v, "[]{}")
                parts := strings.Split(v, "TO")
                if len(parts) == 2 {
                    min := strings.TrimSpace(parts[0])
                    max := strings.TrimSpace(parts[1])
                    if min != "*" && min != "" { out.MinDuration = min }
                    if max != "*" && max != "" { out.MaxDuration = max }
                }
            } else if strings.HasPrefix(v, ">=") || strings.HasPrefix(v, ">") {
                out.MinDuration = strings.TrimLeft(v, ">=")
            } else if strings.HasPrefix(v, "<=") || strings.HasPrefix(v, "<") {
                out.MaxDuration = strings.TrimLeft(v, "<=")
            } else {
                out.MinDuration = v
            }
            continue
        }
        if key == "_time" || key == "time" {
            // Support _time:[start TO end] or shorthand like 15m
            v := strings.TrimSpace(val)
            if strings.HasPrefix(v, "[") || strings.HasPrefix(v, "{") {
                v = strings.Trim(v, "[]{}")
                parts := strings.Split(v, "TO")
                if len(parts) == 2 {
                    start := strings.TrimSpace(parts[0])
                    end := strings.TrimSpace(parts[1])
                    if start != "*" && start != "" { out.StartExpr = start }
                    if end != "*" && end != "" { out.EndExpr = end }
                }
            } else {
                out.Since = v
            }
            continue
        }
        // Normalize tag.* or span_attr.* prefixes into plain key names
        if strings.HasPrefix(key, "tag.") {
            key = strings.TrimPrefix(key, "tag.")
        }
        if strings.HasPrefix(key, "span_attr.") {
            key = strings.TrimPrefix(key, "span_attr.")
        }
        out.Tags[key] = val
    }
    // If nothing parsed, return false
    if out.Service == "" && out.Operation == "" && len(out.Tags) == 0 && out.MinDuration == "" && out.MaxDuration == "" && out.Since == "" {
        return out, false
    }
    return out, true
}

// ---------------- LogsQL translation ----------------

func toLogsQL(q string) (string, bool) {
    // Tokenize and rebuild using:
    // - field:value      => field="value"
    // - field:"phrase"  => field~"phrase"
    // - bareword         => _msg~"bareword"
    // - operators AND/OR/NOT, parentheses preserved
    // - _time:* kept as-is

    s := scanner{src: q}
    out := strings.Builder{}
    first := true
    for {
        t := s.next()
        if t.k == tkEOF { break }
        if !first { out.WriteByte(' ') } else { first = false }
        switch t.k {
        case tkLParen, tkRParen:
            out.WriteString(t.v)
        case tkOp:
            out.WriteString(strings.ToUpper(t.v))
        case tkField:
            // Special-case time filter: normalize Lucene range [start TO end] -> [start,end]
            if strings.EqualFold(t.f, "_time") {
                out.WriteString(t.f)
                out.WriteByte(':')
                val := t.val
                // If bracketed and contains TO, replace TO (case-insensitive) with ','
                vlow := strings.ToLower(val)
                if strings.HasPrefix(val, "[") || strings.HasPrefix(val, "{") {
                    if strings.Contains(vlow, " to ") || strings.Contains(vlow, " to") || strings.Contains(vlow, "to ") {
                        // replace all case-insensitive TO with comma, but only inside the bracket content
                        inner := strings.Trim(val, "[]{}")
                        innerLow := strings.ToLower(inner)
                        // naive split on " to " variants
                        // try the common form with spaces first
                        parts := strings.Split(innerLow, " to ")
                        if len(parts) != 2 {
                            // try no-space variants
                            parts = strings.Split(innerLow, " to")
                            if len(parts) != 2 {
                                parts = strings.Split(innerLow, "to ")
                            }
                        }
                        if len(parts) == 2 {
                            // Recover original substrings by indexes from lowercased inner
                            // Fallback: simple find index and slice from original inner
                            low := innerLow
                            idx := strings.Index(low, "to")
                            if idx >= 0 {
                                left := strings.TrimSpace(inner[:idx])
                                right := strings.TrimSpace(inner[idx+2:])
                                val = string(val[0]) + left + ", " + right + string(val[len(val)-1])
                            }
                        } else {
                            // generic replace of " TO " with ", "
                            repl := strings.ReplaceAll(inner, " TO ", ", ")
                            val = string(val[0]) + repl + string(val[len(val)-1])
                        }
                    }
                }
                if t.quoted {
                    out.WriteByte('"'); out.WriteString(val); out.WriteByte('"')
                } else {
                    out.WriteString(val)
                }
                continue
            }
            // Wildcards/regex -> use ~"..."; otherwise use colon-based equality: key:"value"
            if hasWildcard(t.val) || looksRegex(t.val) {
                out.WriteString(t.f)
                out.WriteString("~\"")
                out.WriteString(wildcardToRegex(t.val))
                out.WriteByte('"')
            } else {
                out.WriteString(t.f)
                out.WriteString(":\"")
                out.WriteString(t.val)
                out.WriteByte('"')
            }
        case tkBare:
            // Bare terms: search in message field
            v := t.v
            if v == "" { continue }
            out.WriteString("_msg~\"")
            out.WriteString(v)
            out.WriteByte('"')
        }
    }
    return strings.TrimSpace(out.String()), true
}

// ---------------- MetricsQL translation ----------------

func toMetricsQL(q string) (string, bool) {
    // Very small subset:
    // - __name__:metric or metric:http_requests_total or bare metric name
    // - AND-joined field:value pairs -> label selectors key="value"
    // - OR/NOT/parentheses are not supported; if present, bail out
    if strings.Contains(q, " OR ") || strings.Contains(q, " or ") ||
        strings.Contains(q, " NOT ") || strings.Contains(q, " not ") ||
        strings.ContainsAny(q, "()") {
        return "", false
    }

    s := scanner{src: q}
    metric := ""
    labels := make([][2]string, 0, 8)

    for {
        t := s.next()
        if t.k == tkEOF { break }
        switch t.k {
        case tkField:
            name := strings.ToLower(t.f)
            if name == "__name__" || name == "metric" {
                if t.val != "" { metric = t.val }
                continue
            }
            // ignore time directives for metrics
            if name == "_time" || name == "time" { continue }
            if t.quoted || hasWildcard(t.val) || looksRegex(t.val) {
                // PromQL label selector supports regex: key=~"..."
                labels = append(labels, [2]string{t.f, "~\"" + wildcardToRegex(t.val) + "\""})
            } else {
                labels = append(labels, [2]string{t.f, "=\"" + t.val + "\""})
            }
        case tkBare:
            // First bare token becomes metric name, others not supported -> ignore
            if metric == "" {
                metric = t.v
            }
        case tkOp:
            // only allow AND as whitespace; explicit ops cause bail-out
            if strings.EqualFold(t.v, "AND") {
                continue
            }
            return "", false
        case tkLParen, tkRParen:
            return "", false
        }
    }

    if metric == "" {
        // Without a metric name, can't form a valid selector
        return "", false
    }

    b := strings.Builder{}
    b.WriteString(metric)
    if len(labels) > 0 {
        b.WriteByte('{')
        for i, kv := range labels {
            if i > 0 { b.WriteByte(',') }
            b.WriteString(kv[0])
            b.WriteString(kv[1])
        }
        b.WriteByte('}')
    }
    return b.String(), true
}

// ---------------- helpers ----------------

type scanner struct {
    src string
    i   int
    n   int
}

func (s *scanner) next() token {
    if s.n == 0 { s.n = len(s.src) }
    // skip whitespace
    for s.i < s.n && unicode.IsSpace(rune(s.src[s.i])) { s.i++ }
    if s.i >= s.n { return token{k: tkEOF} }
    ch := s.src[s.i]
    switch ch {
    case '(':
        s.i++
        return token{k: tkLParen, v: "("}
    case ')':
        s.i++
        return token{k: tkRParen, v: ")"}
    }

    // read a word
    start := s.i
    for s.i < s.n && !unicode.IsSpace(rune(s.src[s.i])) && s.src[s.i] != '(' && s.src[s.i] != ')' {
        // stop at ':' but include for field path
        if s.src[s.i] == ':' { break }
        s.i++
    }
    word := s.src[start:s.i]

    // operator?
    lw := strings.ToLower(word)
    if lw == "and" || lw == "or" || lw == "not" {
        return token{k: tkOp, v: word}
    }

    // field?
    if s.i < s.n && s.src[s.i] == ':' {
        // consume ':'
        s.i++
        // value may be quoted or bare or bracketed
        if s.i < s.n && s.src[s.i] == '"' {
            s.i++
            vStart := s.i
            for s.i < s.n && s.src[s.i] != '"' { s.i++ }
            val := s.src[vStart:s.i]
            if s.i < s.n { s.i++ } // consume closing quote
            return token{k: tkField, f: word, val: val, quoted: true}
        }
        // bracketed range or value (we keep literal for logs and ignore for metrics)
        if s.i < s.n && s.src[s.i] == '[' {
            // scan until closing ']'
            depth := 1
            s.i++
            vStart := s.i
            for s.i < s.n && depth > 0 {
                if s.src[s.i] == '[' { depth++ }
                if s.src[s.i] == ']' { depth--; if depth == 0 { break } }
                s.i++
            }
            val := s.src[vStart:s.i]
            if s.i < s.n { s.i++ } // consume ']'
            return token{k: tkField, f: word, val: "[" + val + "]"}
        }
        // bare value
        vStart := s.i
        for s.i < s.n && !unicode.IsSpace(rune(s.src[s.i])) && s.src[s.i] != '(' && s.src[s.i] != ')' {
            s.i++
        }
        val := s.src[vStart:s.i]
        return token{k: tkField, f: word, val: val}
    }

    // bare word or quoted phrase starting at '"'
    if len(word) == 0 && s.i < s.n && s.src[s.i] == '"' {
        s.i++
        vStart := s.i
        for s.i < s.n && s.src[s.i] != '"' { s.i++ }
        val := s.src[vStart:s.i]
        if s.i < s.n { s.i++ }
        return token{k: tkBare, v: val}
    }

    return token{k: tkBare, v: word}
}

func hasWildcard(s string) bool { return strings.ContainsAny(s, "*?") }
func looksRegex(s string) bool {
    // naive: treat if contains typical regex metachars that aren’t wildcards
    return strings.ContainsAny(s, "[].+|(){}^")
}
func wildcardToRegex(s string) string {
    // convert shell-style wildcards to regex equivalents without anchors
    r := strings.Builder{}
    for i := 0; i < len(s); i++ {
        switch s[i] {
        case '*': r.WriteString(".*")
        case '?': r.WriteByte('.')
        case '.', '+', '(', ')', '[', ']', '{', '}', '|', '^', '$':
            r.WriteByte('\\'); r.WriteByte(s[i])
        default:
            r.WriteByte(s[i])
        }
    }
    return r.String()
}

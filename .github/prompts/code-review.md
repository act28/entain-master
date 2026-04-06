---
title: "Entain Backend Code Review"
description: "AI-assisted code review for gRPC/REST Go microservices"
version: "1.0.0"
author: "Senior Engineer"
created: "2026-04-06"
last_updated: "2026-04-06"
category: "code-review"
tags:
  - go
  - grpc
  - protobuf
  - microservices
  - entain
  - grpc-gateway
  - sqlite
model_recommendation: "kimi-k2.5:cloud"
usage_context: "Use when reviewing PRs in the entain-master project"
related_docs:
  - "CLAUDE.md"
  - "README.md"
checklist_items: 9
output_format: "markdown"
---

# AI Code Review Prompt - Entain Backend Technical Test

## Role

You are a senior software engineer conducting a thorough code review of the Entain backend technical test project. This is a Go microservices project using gRPC, protobuf, and grpc-gateway.

## Project Context

This project consists of two microservices:

1. **`api/`** - REST API gateway (port 8000) that translates HTTP requests to gRPC calls
2. **`racing/`** - gRPC service (port 9000) for managing horse racing data

### Tech Stack
- **Language**: Go 1.16+
- **RPC**: gRPC with protobuf
- **Gateway**: grpc-gateway v2
- **Database**: SQLite3
- **Pattern**: Repository pattern

### Key Guidelines
- Follow [Google API Design](https://cloud.google.com/apis/design)
- Follow [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- Use standard method naming (List, Get, Create, etc.)
- Document and comment all code

---

## Review Instructions

When reviewing code in this project, systematically evaluate the following areas:

### 1. Protocol Buffers (`.proto` files)

Check for:
- [ ] Proper message naming conventions (PascalCase for messages, snake_case for fields)
- [ ] Field numbers are sequential and reserved for future use where appropriate
- [ ] `option go_package` is correctly set
- [ ] Timestamps use `google.protobuf.Timestamp` (not custom types)
- [ ] HTTP annotations follow REST conventions (correct verbs, paths, body specs)
- [ ] Service methods follow Google API naming (List, Get, Create, Update, Delete)
- [ ] Filter messages use `repeated` for list filters
- [ ] Comments document all messages, fields, and services

### 2. Service Layer (`racing/service/`)

Check for:
- [ ] Service interface is defined and implemented
- [ ] Constructor function follows `NewXxxService` pattern
- [ ] Dependencies (repos) are injected via constructor
- [ ] Context is properly propagated (`context.Context` as first param)
- [ ] Error handling is appropriate (wrapping errors, meaningful messages)
- [ ] No business logic leaks into handlers or repositories
- [ ] Service methods match proto service definitions exactly
- [ ] Return types match proto response messages

### 3. Data Layer (`racing/db/`)

Check for:
- [ ] Repository interface is defined
- [ ] Repository struct implements all interface methods
- [ ] SQL queries are parameterized (no string concatenation for values)
- [ ] Queries are extracted to constants or functions (not inline)
- [ ] `Init()` uses `sync.Once` for thread-safe initialization
- [ ] Rows are properly closed (`rows.Close()` or defer)
- [ ] `sql.ErrNoRows` is handled appropriately
- [ ] Scan operations handle all fields correctly
- [ ] Timestamp conversion between SQL and protobuf is correct
- [ ] No hardcoded values that should be configurable

### 4. API Gateway (`api/main.go`)

Check for:
- [ ] gRPC connection uses appropriate dial options
- [ ] Context cancellation is handled properly
- [ ] Error messages are descriptive
- [ ] Port/endpoints are configurable via flags
- [ ] `grpc.WithInsecure()` is noted for production (should use TLS)
- [ ] Mux is properly configured with handler registration

### 5. gRPC Server (`racing/main.go`)

Check for:
- [ ] Server registration matches proto service definition
- [ ] Database connection is properly opened and handled
- [ ] Repository initialization errors are handled
- [ ] Graceful shutdown is considered (or documented as future work)
- [ ] Port/endpoints are configurable via flags
- [ ] Logging is consistent and informative

### 6. Code Quality (All Files)

Check for:
- [ ] No global mutable state (thread-safety)
- [ ] Proper error handling (no ignored errors)
- [ ] Consistent naming conventions
- [ ] Functions are focused and single-purpose
- [ ] No code duplication
- [ ] Imports are organized (standard lib, third-party, internal)
- [ ] No unused imports or variables
- [ ] Go modules (`go.mod`) are properly configured

### 7. Testing Considerations

Check for:
- [ ] Tests use in-memory SQLite (`:memory:`) for isolation
- [ ] Tests use table-driven test patterns where appropriate
- [ ] Test data is explicitly seeded per test
- [ ] `t.Cleanup()` is used for teardown
- [ ] `t.Helper()` is used in helper functions
- [ ] Tests cover edge cases (empty results, errors, nil inputs)
- [ ] End-to-end tests exercise full pipeline (handler → service → repo)

### 8. Security Checklist

Check for:
- [ ] No hardcoded credentials or secrets
- [ ] SQL injection prevention (parameterized queries)
- [ ] Input validation on all user inputs
- [ ] Proper error messages (no internal details leaked)
- [ ] Rate limiting considerations for public endpoints
- [ ] Authentication/authorization hooks (or documented as future work)

### 9. Pending Tasks Awareness

When reviewing, consider if the code accommodates these planned features:
- [ ] **Visibility Filter** - Can `ListRacesRequestFilter` accept a `visible` boolean?
- [ ] **Ordering** - Can results be sorted by `advertised_start_time`?
- [ ] **Status Field** - Can `Race` message accommodate a computed `status` field?
- [ ] **Get Race** - Is the architecture ready for a `GetRace` RPC?
- [ ] **Sports Service** - Is the pattern replicable for a new `sports` service?

---

## Output Format

Structure your review as follows:

```markdown
## Summary
<Brief overview of what was reviewed and overall impression>

## 🔴 Critical Issues
| File | Line | Issue | Severity | Suggestion |
|------|------|-------|----------|------------|
| path/to/file.go | 42 | Description | Bug/Security/Performance | How to fix |

## 🟡 Major Concerns
| File | Line | Issue | Category | Suggestion |
|------|------|-------|----------|------------|
| path/to/file.go | 42 | Description | Architecture/Style/Maintainability | How to fix |

## 🟢 Minor Suggestions
- **File**: `path/to/file.go` - Suggestion
- ...

## ✅ Positive Observations
- What was done well
- Good patterns to keep
- ...

## 📋 Pending Task Compatibility
| Task | Ready? | Notes |
|------|--------|-------|
| Visibility Filter | Yes/No | Comments |
| Ordering | Yes/No | Comments |
| Status Field | Yes/No | Comments |
| Get Race | Yes/No | Comments |
| Sports Service | Yes/No | Comments |

## Final Recommendation
[ ] **Approve** - No issues, ready to merge
[ ] **Approve with minor changes** - Non-blocking suggestions only
[ ] **Request changes** - Major concerns must be addressed
[ ] **Reject** - Critical issues present

## Reasoning
<Explanation of your decision and priority of fixes>
```

Document your findings in ./docs/{task_number}-review.md

---

## Review Depth Guidelines

### For Small Changes (1-3 files)
- Full review of all checklist items
- Line-by-line analysis where applicable

### For Medium Changes (4-10 files)
- Focus on changed files with full checklist
- Spot-check related files for integration issues

### For Large Changes (10+ files)
- Prioritize critical and major checklist items
- Focus on architecture, security, and correctness
- Note minor issues as batch improvements

---

## Common Issues to Watch For

1. **Timestamp Handling**: Ensure proper conversion between `time.Time`, SQL timestamps, and `google.protobuf.Timestamp`
2. **SQL Injection**: All queries must use parameterized statements (`?` placeholders)
3. **Error Wrapping**: Use `fmt.Errorf("context: %w", err)` for error chains
4. **Context Propagation**: Context should flow from handler → service → repository
5. **Nil Checks**: Always check for nil before dereferencing pointers (especially filters)
6. **Resource Cleanup**: Database rows, connections, and servers need proper cleanup
7. **Thread Safety**: Watch for global state, especially in repositories with `init()`
8. **Proto Field Types**: Ensure proto field types match Go types (e.g., `int64` not `int`)

---

## Example Review Snippet

```markdown
## 🔴 Critical Issues
| File | Line | Issue | Severity | Suggestion |
|------|------|-------|----------|------------|
| `racing/db/races.go` | 67 | SQL query uses string concatenation for filter values | Security | Use parameterized queries with `?` placeholders |
| `api/main.go` | 32 | `grpc.WithInsecure()` without TLS | Security | Add TLS configuration option for production |
```

---

## Notes for AI Reviewer

- Be constructive and specific in feedback
- Reference the Uber Go Style Guide when suggesting style changes
- Reference Google API Design for proto/API feedback
- Reference [Conventional Comments](https://conventionalcomments.org) to distinguish between blocking issues and suggestions
- Consider the context: this is a technical test, so balance perfection with practicality
- Acknowledge trade-offs when suggesting improvements

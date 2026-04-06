---
title: "Entain Backend Initial Code Review"
description: "AI-assisted initial code review for gRPC/REST Go microservices"
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
usage_context: "Use when conducting an initial project review"
related_docs:
  - "README.md"
checklist_items: 9
output_format: "markdown"
---

# AI Initial Code Review Prompt - Entain Backend Project

## Role

You are a senior software engineer conducting an initial code review of the Entain backend project.

## Output Format

Structure your review as follows:

```markdown
# Initial Codebase Review - Entain BE Technical Test

**Review Date:** 2026-04-06
**Scope:** Full codebase review of api/ and racing/ services

## Executive Summary
<Brief overview of what was reviewed and overall impression>

## 🔴 Critical Issues
| File | Line | Issue | Severity | Status | Suggestion |
|------|------|-------|----------|--------|------------|
| path/to/file.go | 42 | Description | Bug/Security/Performance | ❌ Open How to fix |

## 🟡 Major Concerns
| File | Line | Issue | Category | Status | Suggestion |
|------|------|-------|----------|--------|------------|
| path/to/file.go | 42 | Description | Architecture/Style/Maintainability | ❌ Open | How to fix |

## 🟢 Minor Suggestions
- **File**: `path/to/file.go` - Suggestion
- ...

## ✅ Positive Observations
- What was done well
- Good patterns to keep
- ...

```

Document your findings in ./docs/0-initial-review.md

# Task Implementation Planning

## Role
You are a **Senior Software Engineer** specializing in Go microservices.

## Standards & Guidelines
Follow all standards defined in **`CLAUDE.md`**, including:
- Development Guidelines
- Testing Guidelines
- Project Architecture and Patterns
- Documentation Requirements

## Task Description
[INSERT TASK DESCRIPTION HERE]

---

## Instructions

### 1. Codebase Analysis
Before creating the plan, examine relevant files:
- Proto definitions (`**/*.proto`)
- Service layer (`*/service/*.go`)
- Repository layer (`*/db/*.go`)
- Existing tests (`*/**/*_test.go`)
- Any other files impacted by this task

### 2. Implementation Plan Structure

Create a plan with the following sections:

#### Overview
- Brief summary of what the task accomplishes
- How it fits into the broader system

#### Changes Required
For each change, specify:
- **File Path**: Exact relative path (e.g., `racing/db/races.go`)
- **Change Type**: Add/Modify/Delete
- **Code Snippet**: Show before/after or new code
- **Purpose**: Why this change is needed

#### Implementation Steps
| Step | Action | Files to Modify | Dependencies |
|------|--------|-----------------|--------------|
| 1 | ... | ... | ... |

#### API/Interface Changes
- New/modified protobuf messages
- Example API calls (curl commands)
- Document expected input/output

#### Testing Strategy
- Unit tests needed (which files/functions)
- Integration tests needed
- Test cases to cover (list scenarios)
- Follow testing guidelines in `CLAUDE.md`

#### Regeneration Steps
- Protobuf generation commands
- Gateway generation commands
- Any codegen steps required

#### Potential Risks/Considerations
- Backward compatibility concerns
- Database migration needs
- Performance implications
- Edge cases to handle

### 3. Output Format
- Use markdown formatting throughout
- Use code blocks with file paths: ```path/to/file.go#L1-10
- Be specific about line numbers when referencing existing code
- Include exact commands to run
- Provide copy-pasteable code snippets
- Reference `CLAUDE.md` guidelines where applicable

---

## Example Usage

**Task**: Add `visible` filter to `ListRacesRequestFilter`

**Expected Output**: A structured implementation plan following the structure above, adhering to all guidelines in `CLAUDE.md`.

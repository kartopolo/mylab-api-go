# Development Documentation

This directory contains internal development documentation for MyLab API developers.

**Audience**: Team members working on the codebase  
**Purpose**: Understand code flow, design patterns, and implementation details  
**NOT for**: API consumers (use `/Docs/api/` instead)

## Structure

### Architecture
- [architecture.md](architecture.md) — Overall system architecture and design patterns

### Feature Flows
Each feature/endpoint has a dedicated flow document explaining execution path:

- [flows/billing-payment-flow.md](flows/billing-payment-flow.md) — Payment save endpoint flow
- [flows/pasien-create-flow.md](flows/pasien-create-flow.md) — Patient create endpoint flow
- [flows/pasien-select-flow.md](flows/pasien-select-flow.md) — Patient select (paged) endpoint flow

## When to Create a Flow Document

Create a new flow document when:
- Implementing a new endpoint/feature
- Modifying existing endpoint logic
- Onboarding new developers to understand a feature

## Flow Document Template

Each flow document must include:

1. **Feature Name & Purpose**
   - What does this feature do?
   - Business context

2. **Entry Point**
   - Handler function location
   - File and line number

3. **Call Sequence**
   - Step-by-step method calls
   - Function names and file locations
   - Show data transformations

4. **Validation Layer**
   - API-layer validation steps
   - What fields are validated?
   - When validation fails?

5. **Service Logic**
   - Business logic implementation
   - Calculations and transformations
   - Database read/write operations

6. **Database Operations**
   - Which tables are affected?
   - SQL operations (INSERT/UPDATE/DELETE)
   - Transaction scope

7. **Error Handling**
   - What can go wrong?
   - Error codes (422/409/500)
   - Recovery/rollback logic

8. **Transaction Boundaries**
   - BEGIN point
   - COMMIT point
   - ROLLBACK conditions

9. **Code References**
   - Actual file paths
   - Line numbers
   - Links to related code

## Standards

- Use **present tense** ("Handler receives...", "Service validates...")
- Include **actual file paths** and line numbers
- Reference **real code**, not pseudocode
- Explain **"why"** not just "what"
- Keep flows **scannable and concise**

## Related Documentation

- **API Documentation**: [../api/README.md](../api/README.md)
- **OpenAPI Contract**: [../openapi/openapi.yaml](../openapi/openapi.yaml)
- **Project README**: [../../README.md](../../README.md)

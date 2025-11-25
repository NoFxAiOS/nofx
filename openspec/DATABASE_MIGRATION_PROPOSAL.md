# Database Migration Proposal

## 1. Proposal Information
| Field | Value |
|-------|-------|
| **Title** | Migrate SQLite to Neon PostgreSQL |
| **Author** | AI Assistant |
| **Date** | 2025-11-25 |
| **Status** | Proposed |
| **Version** | 1.0 |

## 2. Executive Summary
This proposal describes the plan to migrate the application's SQLite database to Neon PostgreSQL to ensure data persistence across backend redeployments. The solution will support dual-database configuration with automatic failover from Neon to SQLite if the cloud database connection fails.

## 3. Background
Currently, the application uses a SQLite database stored in the container's file system. This causes data loss whenever the backend is redeployed because container file systems are ephemeral.

## 4. Objective
- Ensure data persistence across redeployments
- Maintain backward compatibility with existing SQLite configuration
- Implement automatic failover mechanism
- Handle data type and timezone conversion

## 5. Technical Solution

### 5.1 Neon PostgreSQL Configuration
```
DATABASE_URL='postgresql://neondb_owner:npg_i1TxKk6bzZgw@ep-green-poetry-adtqfubw-pooler.c-2.us-east-1.aws.neon.tech/neondb?sslmode=require'
```

### 5.2 Dual-Database Architecture
- **Primary Database**: Neon PostgreSQL
- **Fallback Database**: SQLite
- **Failover Mechanism**: Automatic detection of Neon connection failures

### 5.3 Data Migration
1. Create PostgreSQL tables with compatible schema
2. Migrate existing SQLite data to Neon
3. Verify data integrity

## 6. Implementation Plan

### 6.1 Phase 1: Code Implementation
- Add PostgreSQL driver dependency
- Create migrate.go for data migration
- Modify database.go for dual-database support
- Test API compatibility

### 6.2 Phase 2: Migration Execution
- Deploy updated code
- Execute data migration
- Verify data persistence

### 6.3 Phase 3: Testing
- Test normal database operations
- Test failover to SQLite
- Test redeployment persistence

## 7. Benefits
- **Data Persistence**: No data loss across redeployments
- **High Availability**: Automatic failover ensures continuous operation
- **Scalability**: Neon PostgreSQL can handle growing data volumes
- **Compatibility**: API remains unchanged for existing code

## 8. Risks and Mitigation
| Risk | Mitigation |
|------|------------|
| Neon connection failure | Automatic failover to SQLite |
| Data migration errors | Validate data integrity post-migration |
| SQL syntax incompatibility | Handle placeholder conversion in code |

## 9. Required Resources
- Neon PostgreSQL account and database
- Database migration tools (pgloader)
- Testing environment

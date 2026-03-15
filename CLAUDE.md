# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Custom Instructions

### Communication
- **Primary Language**: 日本語でのやりとりを基本とする (Use Japanese as the primary communication language)

### External Content Management
- **MCP Server Changes**: MCPサーバーで外部コンテンツに変更を加える時は、必ず事前に確認をとること (Always seek confirmation before making changes to external content via MCP servers)

### Git Commit Guidelines
- **Commit Granularity**: gitコミットは適切な粒度にわけて簡潔なコミットメッセージにより行うこと (Make git commits with appropriate granularity and concise commit messages)

### Communication Style
- **Ask Questions**: 指示が曖昧だったり、不明点・懸念点があれば遠慮なく質問・確認すること (Don't hesitate to ask questions or seek clarification when instructions are ambiguous or when there are unclear points or concerns)

## Project Overview

YouTube Study Space is a 24/7 automated online study room livestreamed on YouTube. Users can join and leave virtual study sessions through YouTube live chat commands. The system supports both general seats and member-exclusive seats, tracks work time, and provides moderation and automation features.

## Development Commands

### Go Backend (`system/`)
```bash
# Run the main bot locally
go run main.go

# Run all tests with randomization
go test -shuffle=on -v ./...

# Run tests for a specific package
go test ./core/youtubebot/...

# Generate repository mocks
mockgen -source ./core/repository/interface.go -destination ./core/repository/mocks/interface.go -package mock_repository

# Update dependencies
go mod tidy

# Regenerate typed i18n wrappers
go generate ./...
```

### Frontend (`youtube-monitor/`)
```bash
# Install dependencies
pnpm install

# Development server
pnpm dev

# Production build
pnpm build

# Production server
pnpm start

# Run tests
pnpm test

# Linting and formatting (using Biome)
pnpm format
pnpm lint
pnpm lint:fix
pnpm format:fix

# Storybook for component development
pnpm storybook
pnpm build-storybook
```

### AWS Infrastructure (`aws-cdk/`)
```bash
# Install dependencies
pnpm install

# Preview infrastructure changes
pnpm cdk:diff

# Deploy infrastructure
pnpm cdk:deploy

# Build CDK code
pnpm build

# Run CDK tests
pnpm test
```

### Documentation Site (`docs-site/`)
```bash
# Install dependencies
pnpm install

# Local development server
pnpm start

# Build static site
pnpm build

# Formatting and linting
pnpm format
pnpm lint
```

- `youtube-monitor/`, `aws-cdk/`, and `docs-site/` use `pnpm` (`packageManager: pnpm@10.4.0`).
- Do not use `npm` or `yarn` for these directories unless explicitly required.

## Architecture

### High-Level Structure
- **`system/`** - Go backend, scheduled jobs, and AWS Lambda entrypoints
- **`youtube-monitor/`** - Next.js frontend for the study room interface
- **`aws-cdk/`** - AWS infrastructure as code
- **`docs-site/`** - Docusaurus documentation site with i18n support
- **`firebase/`** - Firestore configuration

### Core Architecture Patterns

**Event-Driven Serverless**:
- **Every 1 minute**: `youtube_organize_database` + `check_live_stream_status`
- **Every 15 minutes**: `update_work_name_trend`
- **Daily at 00:00 JST**: EventBridge Scheduler starts `start_daily_batch`, which launches the Step Functions and Fargate daily batch flow

**Multi-Database Strategy**:
- **Firestore**: Real-time user sessions, room state, chat history, configuration
- **DynamoDB**: Configuration and secret lookup for Lambda-side operations
- **BigQuery**: Historical analytics and archival processing
- **Cloud Storage**: Transfer source for BigQuery import jobs

**Command Processing Flow**:
1. YouTube Live Chat API retrieves messages
2. Commands are parsed and validated
3. `workspaceapp` processes seat, break, user, and moderation behavior
4. Firestore transactions persist state changes
5. Replies and notifications are posted to YouTube Live Chat and Discord when needed

### Key Components

**Backend (`system/core/`)**:
- `workspaceapp/` - Main application layer for command handling, presenters, validation, and batch jobs
- `youtubebot/` - YouTube Live Chat API integration
- `repository/` - Firestore data access layer
- `guardians/` - Live stream monitoring and guard logic
- `moderatorbot/` - Moderation and Discord notification integrations
- `mybigquery/` - BigQuery transfer logic
- `i18n/` - Locales and typed translation wrappers

**Backend Entrypoints (`system/`)**:
- `main.go` - Local live chat bot runner
- `cmd/batch/` - Daily batch executable for Fargate
- `aws-lambda/` - Lambda handlers such as `youtube_organize_database`, `check_live_stream_status`, `start_daily_batch`, `set_desired_max_seats`, and `update_work_name_trend`

**Frontend Architecture**:
- **Next.js** with TypeScript
- **Redux Toolkit** for state management
- **Emotion** for CSS-in-JS styling
- **SWR** for data fetching
- **Framer Motion** for animations
- **next-i18next** for localization

## Data Models

### Core Entities
- **SeatDoc**: Seat information such as user ID, entry time, and work content
- **UserDoc**: User profiles and accumulated activity
- **ConstantsConfigDoc**: Runtime constants such as seat counts and polling settings
- **CredentialsConfigDoc**: Authentication and configuration references

### State Management
- Firestore transactions ensure atomicity for seat assignment and updates
- User sessions persist across reconnections
- Frontend consumes real-time data via Firebase and SWR-based flows

## Testing Strategy

### Go Backend
- Standard Go testing with `testify`
- Mock generation using `go.uber.org/mock`
- Package-level tests around command parsing, repository behavior, and workspace flows

### Frontend
- **Jest** with **React Testing Library**
- **ts-jest** for TypeScript support
- **jsdom** environment for DOM testing
- Component development and verification with **Storybook**

### Infrastructure
- CDK unit tests for schedule and infrastructure invariants
- GitHub Actions CI/CD pipeline

## Code Guidelines

### Important Conventions
- **Preserve `NOTE` comments**: These contain important implementation details and should not be removed
- **Add review comments**: Use `[NOTE FOR REVIEW]` prefix for temporary explanatory comments
- **Error handling**: Include contextual information in error messages
- **Transaction safety**: Use Firestore transactions for data consistency

### Command System
Representative chat commands:
- `!in` - General seat entry
- `/in` - Member seat entry
- `!out` - Exit seat
- `!break` / `!rest` / `!chill` - Start break
- `!resume` - End break
- `!my` - Show personal stats or update personal settings
- `!rank` - Display rankings
- `!more` / `!okawari` - Extend work time
- Moderation: `!kick`, `!block`

### Internationalization
- Support for English, Japanese, and Korean locale files
- Message templates live under `system/core/i18n/`
- Typed wrappers are generated from locale metadata with `go generate ./...`
- Frontend uses `next-i18next`

## Development Environment

### Required Setup
1. Go 1.24.0+ for backend development
2. Node.js 18+ and `pnpm` for frontend and TypeScript-based subprojects
3. Google Cloud credentials for Firestore, Cloud Storage, and BigQuery access
4. AWS credentials for Lambda, Step Functions, Scheduler, and CDK operations
5. YouTube Data API credentials

### Local Development
- `system/main.go` runs the live chat bot locally
- Install frontend dependencies with `pnpm install` in `youtube-monitor/`
- Start the frontend dev server with `pnpm dev` in `youtube-monitor/`
- Start the frontend production server with `pnpm start` in `youtube-monitor/` on port `18080`
- Storybook runs on port `6006`
- Use `.env` files for local configuration

### Deployment
- AWS Lambda functions are containerized with `system/Dockerfile.lambda`
- Daily batch jobs run on ECS Fargate via `system/Dockerfile.fargate`
- Daily orchestration is driven by EventBridge Scheduler and Step Functions
- Infrastructure changes are managed through `aws-cdk/`

## External Integrations

### Google Services
- **YouTube Data API v3**: Live chat message retrieval and posting
- **Firestore**: Primary real-time datastore
- **BigQuery**: Analytics and history transfer target
- **Cloud Storage**: Export and transfer staging

### AWS Services
- **Lambda**: Scheduled and on-demand compute
- **EventBridge / EventBridge Scheduler**: Periodic execution and daily scheduling
- **Step Functions**: Daily batch orchestration
- **ECS Fargate**: Daily batch runtime
- **DynamoDB**: Configuration storage for Lambda functions
- **API Gateway**: Authenticated REST API

### Third-Party
- **Discord**: Notification webhooks for moderation and failures
- **OpenAI API**: Work name trend generation
- **FOSSA**: License compliance scanning

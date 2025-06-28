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

YouTube Study Space is a 24/7 automated online study room livestreamed on YouTube. Users can join/leave virtual study sessions through YouTube live chat commands, featuring a fully automated entry/exit system with unlimited capacity. The system supports both general seats and member-exclusive seats, with comprehensive user management, work time tracking, and moderation features.

## Development Commands

### Go Backend (`system/`)
```bash
# Run the main bot locally
go run main.go

# Run all tests with randomization
go test -shuffle=on -v ./...

# Run tests for specific package
go test ./core/youtubebot/...

# Generate mocks
mockgen -source=core/repository/firestore_controller_interface.go -destination=core/repository/mocks/firestore_controller_interface.go -package=mock_myfirestore

# Update dependencies
go mod tidy
```

### Frontend (`youtube-monitor/`)
```bash
# Development server
npm run dev

# Production build
npm run build

# Run tests
npm run test

# Linting and formatting (using Biome)
npm run lint
npm run lint:fix
npm run format:fix

# Storybook for component development
npm run storybook
npm run build-storybook
```

### AWS Infrastructure (`aws-cdk/`)
```bash
# Deploy infrastructure
cdk deploy

# Preview infrastructure changes
cdk diff

# Build CDK code
npm run build

# Run CDK tests
npm run test
```

### Documentation Site (`docs-site/`)
```bash
# Local development server
npm start

# Build static site
npm run build

# Formatting and linting
npm run format
npm run lint
```

## Architecture

### High-Level Structure
- **`system/`** - Go backend with core business logic, YouTube bot, and Lambda functions
- **`youtube-monitor/`** - Next.js frontend for the study room interface
- **`aws-cdk/`** - Infrastructure as Code for AWS resources
- **`docs-site/`** - Docusaurus documentation site with i18n support
- **`firebase/`** - Firestore database configuration

### Core Architecture Patterns

**Event-Driven Serverless**: The system uses AWS Lambda functions triggered by EventBridge for scheduled tasks:
- **1-minute interval**: `youtube_organize_database` + `check_live_stream_status`
- **Daily at midnight JST**: `daily_organize_database`
- **Daily at 1 AM JST**: `transfer_collection_history_bigquery`

**Multi-Database Strategy**:
- **Firestore**: Real-time user sessions, room state, chat history
- **DynamoDB**: Configuration secrets for AWS Lambda functions
- **BigQuery**: Historical analytics and data warehousing

**Command Processing Flow**:
1. YouTube Live Chat API retrieves messages
2. Commands are parsed and validated
3. Firestore transactions ensure data consistency
4. Responses are sent back to YouTube Live Chat
5. Discord notifications for moderation events

### Key Components

**Backend (`system/core/`)**:
- `system.go` - Main command processing and seat management logic
- `youtubebot/` - YouTube Live Chat API integration
- `repository/` - Data access layer with Firestore controllers
- `guardians/` - Security and anti-spam systems
- `moderatorbot/` - Moderation features (kick, block, etc.)
- `mybigquery/` - Analytics data pipeline
- `i18n/` - Multi-language support (EN, JA, KO)

**Frontend Architecture**:
- **Next.js** with TypeScript for type safety
- **Redux Toolkit** for state management
- **Emotion** for CSS-in-JS styling
- **SWR** for data fetching
- **Framer Motion** for animations
- **React H5 Audio Player** with music metadata parsing

**Batch Processing System**: See the detailed batch design documentation in Notion for:
- OrganizeDB (1-minute interval): User state management, auto-exit, break resumption
- DailyOrganizeDB: Daily stats reset and RP processing coordination
- Parallel RP Processing: Scalable user ranking point calculations
- BigQuery Transfer: Historical data archival from Firestore

## Data Models

### Core Entities
- **SeatDoc**: Seat information (user ID, entry time, work content, etc.)
- **UserDoc**: User profiles (total study time, achievements, etc.)
- **ConstantsConfigDoc**: System constants (max seats, polling intervals)
- **CredentialsConfigDoc**: Authentication credentials

### State Management
- Firestore transactions ensure atomicity for seat assignments
- User sessions persist across reconnections
- Real-time updates via Firestore listeners in frontend

## Testing Strategy

### Go Backend
- Standard Go testing with `testify` assertions
- Mock generation using `go.uber.org/mock`
- Integration tests with Firestore emulator
- Test coverage for critical business logic

### Frontend
- **Jest** with **React Testing Library**
- **ts-jest** for TypeScript support
- **jsdom** environment for DOM testing
- Component testing with **Storybook**

### Infrastructure
- CDK unit tests for infrastructure validation
- GitHub Actions CI/CD pipeline

## Code Guidelines

### Important Conventions
- **Preserve `NOTE` comments**: These contain important implementation details and should not be removed
- **Add review comments**: Use `[NOTE FOR REVIEW]` prefix for temporary explanatory comments
- **Error handling**: Include contextual information in error messages
- **Transaction safety**: Use Firestore transactions for data consistency

### Command System
The system recognizes these chat commands:
- `!in` - General seat entry
- `/in` - Member seat entry  
- `!out` - Exit seat
- `!break`/`!rest`/`!chill` - Start break
- `!resume` - End break
- `!my` - Show personal stats
- `!rank` - Display rankings
- `!more`/`!okawari` - Extend work time
- Moderation: `!kick`, `!block` (moderator only)

### Internationalization
- Support for English, Japanese, Korean
- Message templates in `core/i18n/`
- Frontend uses `next-i18next`

## Development Environment

### Required Setup
1. Go 1.23+ for backend development
2. Node.js 18+ for frontend and infrastructure
3. Google Cloud credentials for Firestore/BigQuery access
4. AWS credentials for Lambda deployment
5. YouTube Data API credentials

### Local Development
- The `system/main.go` runs the live chat bot locally
- Frontend runs on port 18080 (`npm start`)
- Storybook runs on port 6006
- Use `.env` files for local configuration

### Deployment
- AWS Lambda functions are containerized and deployed via CDK
- Frontend can be deployed as static site or Next.js app
- Documentation site deploys to GitHub Pages
- Infrastructure changes require CDK deployment

## External Integrations

### Google Services
- **YouTube Data API v3**: Live chat message retrieval and posting
- **Firestore**: Primary database for real-time data
- **BigQuery**: Analytics and historical data storage
- **Cloud Storage**: File storage for exports and backups

### AWS Services
- **Lambda**: Serverless compute for batch operations
- **EventBridge**: Scheduled triggers for automation
- **DynamoDB**: Configuration storage for Lambda functions
- **API Gateway**: REST API with authentication

### Third-Party
- **Discord**: Notification webhooks for moderation events
- **FOSSA**: License compliance scanning
import { test } from '@playwright/test';

// Mock data that represents a realistic UI state.
const MOCK_REPO = {
	id: 'repo_mock01',
	owner: 'acme',
	name: 'webapp',
	full_name: 'acme/webapp',
	summary: '',
	tech_stack: [],
	setup_status: 'ready',
	has_code: true,
	has_claude_md: false,
	has_readme: true,
	expectations: '',
	setup_completed_at: '2025-01-15T10:05:00Z',
	created_at: '2025-01-15T10:00:00Z'
};

// Repo variant: pending setup (pre-existing repo, setup_status = 'pending')
const MOCK_REPO_PENDING = {
	...MOCK_REPO,
	setup_status: 'pending',
	setup_completed_at: undefined
};

// Repo variant: scanning in progress (setup_status = 'scanning')
const MOCK_REPO_SCANNING = {
	...MOCK_REPO,
	setup_status: 'scanning',
	setup_completed_at: undefined
};

// Repo variant: scan complete, needs user configuration (setup_status = 'needs_setup')
const MOCK_REPO_NEEDS_SETUP = {
	...MOCK_REPO,
	setup_status: 'needs_setup',
	summary:
		'A full-stack web application built with SvelteKit and Express. The frontend uses Tailwind CSS for styling with a component library based on shadcn-svelte. The backend API serves RESTful endpoints backed by PostgreSQL with Drizzle ORM. The project includes comprehensive test coverage using Vitest and Playwright for E2E tests.',
	tech_stack: [
		'TypeScript',
		'SvelteKit',
		'Tailwind CSS',
		'Express',
		'PostgreSQL',
		'Drizzle ORM',
		'Vitest',
		'Playwright',
		'Docker',
		'pnpm'
	],
	has_code: true,
	has_claude_md: true,
	has_readme: true,
	expectations: '',
	setup_completed_at: undefined
};

// Repo variant: scan complete but no tech stack detected (empty repo scenario)
const MOCK_REPO_NEEDS_SETUP_EMPTY = {
	...MOCK_REPO,
	setup_status: 'needs_setup',
	summary: 'Empty repository with no source code detected.',
	tech_stack: [],
	has_code: false,
	has_claude_md: false,
	has_readme: false,
	expectations: '',
	setup_completed_at: undefined
};

// Repo variant: AI is reviewing user's configuration (setup_status = 'configuring')
const MOCK_REPO_CONFIGURING = {
	...MOCK_REPO_NEEDS_SETUP,
	setup_status: 'configuring',
	expectations:
		'## Code Quality\n- Follow ESLint rules\n- Use Prettier for formatting\n\n## Testing\n- Write unit tests for new functions\n- Use Vitest'
};

// Sample agent logs that showcase all the different log types and syntax highlighting.
// These are used by the running/review task detail screenshots so we can preview how
// the terminal rendering looks for each prefix and inline formatting rule.
const SAMPLE_LOGS_RUNNING: Record<number, string[]> = {
	1: [
		'[system] Task tsk_running01 claimed by worker w_abc123',
		'[system] Spinning up agent container verve:base',
		'[system] Container started in 2.4s — attached to stdout/stderr',
		'',
		'[agent] Analyzing codebase structure...',
		'[agent] Reading config at src/db/pool.ts',
		'[agent] Found 3 files related to connection pooling',
		'',
		'[claude] I can see the connection pool is configured in src/db/pool.ts with a max of 5 connections.',
		'[claude] The issue is that each request creates a new pool instead of reusing the shared instance.',
		'[claude] Let me trace through the request lifecycle to understand the full flow:',
		'[claude]   1. src/server.ts creates the Express app',
		'[claude]   2. src/middleware/db.ts attaches a pool to each request',
		'[claude]   3. src/db/pool.ts exports createPool() which is called per-request',
		'[claude] The fix is to call createPool() once at startup and share it via middleware.',
		'',
		'[agent] Editing src/db/pool.ts',
		'$ git diff --stat',
		' src/db/pool.ts       | 18 +++++++++-------',
		' src/middleware/db.ts  |  4 ++--',
		' 2 files changed, 12 insertions(+), 10 deletions(-)',
		'',
		'[claude] I\'ve refactored the pool to be a singleton. Now testing the change:',
		'',
		'[agent] Running test suite',
		'$ npm test',
		'  PASS src/db/pool.test.ts (3 tests, 0.8s)',
		'  PASS src/middleware/db.test.ts (5 tests, 1.2s)',
		'  PASS src/routes/users.test.ts (12 tests, 2.1s)',
		'',
		'[info] All 20 tests passed in 4.1s',
		'[claude] Tests are passing. The connection pool is now properly shared across requests.',
		'[claude] Under load this should reduce connection count from O(n) to a fixed pool of 5.',
		'',
		'[agent] Creating commit "fix: share database connection pool across requests"',
		'[agent] Pushing to branch verve/fix-db-pool',
		'[system] Agent completed successfully'
	]
};

const SAMPLE_LOGS_REVIEW: Record<number, string[]> = {
	1: [
		'[system] Task tsk_review01 claimed by worker w_xyz789',
		'[system] Container verve:base ready',
		'',
		'[agent] Starting work on dark mode support',
		'[agent] Scanning project for existing theme infrastructure',
		'',
		'[claude] The project uses Tailwind CSS with a custom color palette defined in tailwind.config.js.',
		'[claude] There\'s no existing dark mode setup. I\'ll implement this using Tailwind\'s "class" strategy',
		'[claude] so we can toggle dark mode programmatically and persist the user\'s preference.',
		'',
		'[agent] Creating src/lib/stores/theme.ts',
		'[agent] Updating src/app.html to add class to <html> element',
		'[agent] Modifying src/lib/components/Header.svelte — adding toggle button',
		'',
		'[claude] I need to handle 3 preference states:',
		'[claude]   - "light" — always light',
		'[claude]   - "dark" — always dark',
		'[claude]   - "system" — follow OS preference via matchMedia("(prefers-color-scheme: dark)")',
		'[claude] Storing the choice in localStorage under key "theme-preference".',
		'',
		'[agent] Writing theme store implementation',
		'$ cat src/lib/stores/theme.ts',
		'  export type ThemePreference = "light" | "dark" | "system";',
		'  export const theme = writable<ThemePreference>("system");',
		'',
		'[warn] tailwind.config.js: darkMode is set to "media" — changing to "class"',
		'[agent] Updated tailwind.config.js',
		'',
		'[claude] Now adding the toggle component. Using a sun/moon icon pair with smooth transition.',
		'',
		'[agent] Running build to verify no errors',
		'$ npm run build',
		'  vite v5.2.0 building for production...',
		'  143 modules transformed',
		'  dist/index.html   0.45 kB gzip',
		'  dist/assets/app.js   82.3 kB gzip',
		'  Build completed in 3.8s',
		'',
		'[info] Build succeeded — 0 errors, 0 warnings',
		'[agent] Running tests',
		'$ npm test',
		'  PASS src/lib/stores/theme.test.ts (4 tests)',
		'  PASS src/lib/components/Header.test.ts (6 tests)',
		'',
		'[claude] Everything looks good. The theme toggle is working correctly with all 3 modes.',
		'[claude] Dark mode colors are applied via Tailwind\'s dark: prefix throughout the app.',
		'',
		'[agent] Creating commit and pushing to verve/add-dark-mode',
		'[agent] Creating pull request #42: "Add dark mode support with system preference detection"',
		'[info] PR created: https://github.com/acme/webapp/pull/42',
		'[system] Agent completed — PR ready for review'
	]
};

const SAMPLE_LOGS_RETRY_RUNNING: Record<number, string[]> = {
	1: [
		'[system] Task tsk_retry_running01 claimed by worker w_def456',
		'[agent] Analyzing flaky test failures',
		'[claude] The integration tests are failing intermittently due to database connection teardown race conditions.',
		'[agent] Attempting fix in src/tests/helpers/db-setup.ts',
		'[error] Test suite failed: 3 of 15 tests timed out',
		'[agent] Fix did not resolve all failures',
		'[system] Agent completed with failures'
	],
	2: [
		'[system] Retry attempt 2 — task tsk_retry_running01 claimed by worker w_def456',
		'[system] Container verve:base ready',
		'',
		'[agent] Re-analyzing test failures from attempt 1',
		'[agent] Reading CI logs from previous run',
		'',
		'[claude] Looking at the previous failure more carefully, the root cause is not just the teardown.',
		'[claude] The tests share a single database instance and run in parallel, causing lock contention.',
		'[claude] I need to:',
		'[claude]   1. Give each test file its own isolated database',
		'[claude]   2. Fix the connection teardown to use proper async cleanup',
		'[claude]   3. Add a retry wrapper for transient connection errors',
		'',
		'[agent] Editing src/tests/helpers/db-setup.ts',
		'[agent] Editing src/tests/integration/users.test.ts',
		'[agent] Editing src/tests/integration/orders.test.ts',
		'',
		'[claude] Each test file now gets a unique database via template databases. This eliminates',
		'[claude] the parallel execution conflicts entirely.',
		'',
		'$ npm run test:integration',
		'  PASS src/tests/integration/users.test.ts (8 tests, 3.2s)',
		'  PASS src/tests/integration/orders.test.ts (5 tests, 2.8s)',
		'  PASS src/tests/integration/auth.test.ts (2 tests, 1.1s)',
		'',
		'[info] All 15 integration tests passed — 0 flakes across 3 consecutive runs',
		'[agent] Pushing changes to verve/fix-flaky-tests',
		'[system] Agent completed successfully — PR #45 updated'
	]
};

const SAMPLE_LOGS_FAILED: Record<number, string[]> = {
	1: [
		'[system] Task tsk_failed01 claimed by worker w_ghi789',
		'[agent] Starting ORM migration from raw SQL to Drizzle',
		'[agent] Found 24 query files in src/db/queries/',
		'[claude] This is a large migration. I\'ll work through the queries module by module.',
		'[agent] Migrating src/db/queries/users.ts',
		'[agent] Migrating src/db/queries/orders.ts',
		'[error] Schema incompatibility: orders table uses a composite key that Drizzle doesn\'t support natively',
		'[claude] The orders table has a composite primary key (user_id, order_id) which requires a workaround in Drizzle.',
		'[claude] I\'ll use the composite primaryKey helper from drizzle-orm/pg-core.',
		'[agent] Applied workaround for composite keys',
		'$ npm test',
		'  FAIL src/db/queries/orders.test.ts',
		'    TypeError: Cannot read properties of undefined (reading \'id\')',
		'  FAIL src/db/queries/users.test.ts',
		'    Error: relation "users_new" does not exist',
		'[error] 8 of 24 tests failed — migration incomplete',
		'[system] Agent completed with failures'
	],
	2: [
		'[system] Retry attempt 2 — task tsk_failed01 claimed by worker w_ghi789',
		'[agent] Resuming ORM migration — addressing 8 test failures',
		'[claude] The previous attempt had 2 issues:',
		'[claude]   1. The composite key workaround was incorrect — need to use sql`` template',
		'[claude]   2. Migration created "users_new" table instead of replacing "users"',
		'[agent] Fixing schema definitions',
		'[error] Drizzle introspection failed: incompatible schema version 0.28 (expected >= 0.30)',
		'[error] Cannot proceed — Drizzle ORM version in package.json is outdated',
		'[system] Agent failed — incompatible schema version'
	]
};

const MOCK_TASKS = [
	{
		id: 'tsk_pending01',
		number: 1,
		repo_id: 'repo_mock01',
		title: 'Add user authentication',
		description: 'Implement JWT-based auth with login/signup pages',
		status: 'pending',
		logs: [],
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: ['Login page works', 'JWT tokens issued'],
		consecutive_failures: 0,
		cost_usd: 0,
		skip_pr: false,
		ready: true,
		created_at: '2025-06-01T09:00:00Z',
		updated_at: '2025-06-01T09:00:00Z'
	},
	{
		id: 'tsk_notready01',
		number: 2,
		repo_id: 'repo_mock01',
		title: 'Refactor payment processing module',
		description: 'Break up the monolithic payment handler into smaller services',
		status: 'pending',
		logs: [],
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: ['Payment tests pass', 'No regressions'],
		depends_on: ['tsk_pending01'],
		consecutive_failures: 0,
		cost_usd: 0,
		skip_pr: false,
		ready: false,
		created_at: '2025-06-01T08:00:00Z',
		updated_at: '2025-06-01T08:00:00Z'
	},
	{
		id: 'tsk_running01',
		number: 3,
		repo_id: 'repo_mock01',
		title: 'Fix database connection pooling',
		description: 'Connection pool exhaustion under load',
		status: 'running',
		logs: [],
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: [],
		consecutive_failures: 0,
		cost_usd: 0.12,
		skip_pr: false,
		started_at: '2025-06-01T10:30:00Z',
		created_at: '2025-06-01T10:00:00Z',
		updated_at: '2025-06-01T10:30:00Z'
	},
	{
		id: 'tsk_review01',
		number: 4,
		repo_id: 'repo_mock01',
		title: 'Add dark mode support',
		description: 'Implement theme toggle with system preference detection',
		status: 'review',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/42',
		pr_number: 42,
		branch_name: 'verve/add-dark-mode',
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: ['Theme toggle in header', 'Persists preference'],
		consecutive_failures: 0,
		cost_usd: 0.45,
		skip_pr: false,
		started_at: '2025-06-01T08:00:00Z',
		duration_ms: 180000,
		created_at: '2025-06-01T07:00:00Z',
		updated_at: '2025-06-01T08:03:00Z'
	},
	{
		id: 'tsk_merged01',
		number: 5,
		repo_id: 'repo_mock01',
		title: 'Update API documentation',
		description: 'Auto-generate OpenAPI spec from route handlers',
		status: 'merged',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/38',
		pr_number: 38,
		branch_name: 'verve/update-api-docs',
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: [],
		consecutive_failures: 0,
		cost_usd: 0.30,
		skip_pr: false,
		started_at: '2025-05-30T14:00:00Z',
		duration_ms: 120000,
		created_at: '2025-05-30T13:00:00Z',
		updated_at: '2025-05-30T14:02:00Z'
	},
	{
		id: 'tsk_failed01',
		number: 6,
		repo_id: 'repo_mock01',
		title: 'Migrate to new ORM',
		description: 'Replace raw SQL with Drizzle ORM',
		status: 'failed',
		logs: [],
		attempt: 2,
		max_attempts: 3,
		acceptance_criteria: ['All queries migrated', 'Tests pass'],
		consecutive_failures: 1,
		cost_usd: 0.85,
		skip_pr: false,
		started_at: '2025-05-29T16:00:00Z',
		duration_ms: 300000,
		created_at: '2025-05-29T15:00:00Z',
		updated_at: '2025-05-29T16:05:00Z'
	},
	// Retry scenario: agent is actively running a retry on a task that already has a PR
	{
		id: 'tsk_retry_running01',
		number: 7,
		repo_id: 'repo_mock01',
		title: 'Fix flaky integration tests',
		description: 'Stabilize integration test suite that randomly fails in CI',
		status: 'running',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/45',
		pr_number: 45,
		branch_name: 'verve/fix-flaky-tests',
		attempt: 2,
		max_attempts: 3,
		acceptance_criteria: ['All integration tests pass consistently', 'No test timeouts'],
		retry_reason: 'CI checks failed — test suite still flaky after first attempt',
		consecutive_failures: 1,
		cost_usd: 0.62,
		skip_pr: false,
		started_at: '2025-06-01T11:00:00Z',
		created_at: '2025-06-01T09:00:00Z',
		updated_at: '2025-06-01T11:05:00Z'
	},
	// Retry scenario: task is pending (waiting for agent pickup) with existing PR
	{
		id: 'tsk_retry_pending01',
		number: 8,
		repo_id: 'repo_mock01',
		title: 'Improve error handling in API',
		description: 'Add proper error responses and validation to all API endpoints',
		status: 'pending',
		logs: [],
		pull_request_url: 'https://github.com/acme/webapp/pull/47',
		pr_number: 47,
		branch_name: 'verve/improve-error-handling',
		attempt: 3,
		max_attempts: 3,
		acceptance_criteria: ['All endpoints return proper error codes', 'Input validation on all routes'],
		retry_reason: 'Missing validation on POST /users endpoint',
		retry_context: 'FAIL: TestPostUsers_InvalidInput\n  Expected status 400, got 500\n  Error: missing required field validation',
		consecutive_failures: 2,
		cost_usd: 1.20,
		skip_pr: false,
		created_at: '2025-05-31T14:00:00Z',
		updated_at: '2025-06-01T12:00:00Z'
	}
];

// Map of task ID to per-attempt logs for the SSE mock.
const MOCK_TASK_LOGS: Record<string, Record<number, string[]>> = {
	tsk_running01: SAMPLE_LOGS_RUNNING,
	tsk_review01: SAMPLE_LOGS_REVIEW,
	tsk_retry_running01: SAMPLE_LOGS_RETRY_RUNNING,
	tsk_failed01: SAMPLE_LOGS_FAILED
};

// --- Mock Epic Data ---

// Epic in draft state — no planning session started yet.
const MOCK_EPIC_DRAFT = {
	id: 'epc_draft01',
	repo_id: 'repo_mock01',
	number: 1,
	title: 'Implement user authentication system',
	description:
		'Build a complete authentication system with **JWT tokens**, login/signup pages, password reset flow, and role-based access control.\n\n## Requirements\n- Integrate with the existing Express API\n- Use PostgreSQL for user storage\n- Support `bcrypt` password hashing\n\nSee the [design doc](https://example.com) for details.',
	status: 'draft',
	proposed_tasks: [],
	task_ids: [],
	session_log: [],
	not_ready: false,
	created_at: '2025-06-01T09:00:00Z',
	updated_at: '2025-06-01T09:00:00Z'
};

// Epic in planning state — agent has proposed tasks and there's an active session.
const MOCK_EPIC_PLANNING = {
	id: 'epc_planning01',
	repo_id: 'repo_mock01',
	number: 2,
	title: 'Add real-time notifications',
	description:
		'Implement a real-time notification system using WebSockets. Users should receive notifications for task completions, PR reviews, and system alerts. Include a notification bell in the header with an unread count badge.',
	status: 'planning',
	proposed_tasks: [
		{
			temp_id: 'tmp_01',
			title: 'Set up WebSocket server infrastructure',
			description:
				'Create a WebSocket server using `ws` library integrated with the Express app.\n\n**Key tasks:**\n1. Handle connection lifecycle\n2. Authentication via JWT\n3. Heartbeat pings every 30s',
			depends_on_temp_ids: [],
			acceptance_criteria: [
				'WebSocket server accepts connections on /ws',
				'Connections require valid JWT token',
				'Heartbeat ping/pong every 30s'
			]
		},
		{
			temp_id: 'tmp_02',
			title: 'Create notification data model and API',
			description:
				'Design the notifications table in PostgreSQL with fields for type, message, read status, and user association. Add REST endpoints for listing and marking notifications as read.',
			depends_on_temp_ids: [],
			acceptance_criteria: [
				'Notifications table with proper indexes',
				'GET /notifications returns paginated results',
				'PATCH /notifications/:id/read marks as read'
			]
		},
		{
			temp_id: 'tmp_03',
			title: 'Implement notification dispatch service',
			description:
				'Create a service that listens for system events (task complete, PR review, etc.) and dispatches notifications to the correct users via WebSocket and database persistence.',
			depends_on_temp_ids: ['tmp_01', 'tmp_02'],
			acceptance_criteria: [
				'Dispatches on task_completed event',
				'Dispatches on pr_review_requested event',
				'Falls back to database if user offline'
			]
		},
		{
			temp_id: 'tmp_04',
			title: 'Build notification bell UI component',
			description:
				'Add a notification bell icon to the app header with unread count badge. Clicking opens a dropdown with recent notifications. Each notification is clickable and navigates to the relevant resource.',
			depends_on_temp_ids: ['tmp_02', 'tmp_03'],
			acceptance_criteria: [
				'Bell icon shows unread count',
				'Dropdown lists last 20 notifications',
				'Clicking notification navigates to resource',
				'Mark as read on click'
			]
		},
		{
			temp_id: 'tmp_05',
			title: 'Add notification preferences page',
			description:
				'Create a settings page where users can configure which notification types they want to receive and their preferred delivery method (in-app, email, or both).',
			depends_on_temp_ids: ['tmp_03'],
			acceptance_criteria: [
				'Settings page lists all notification types',
				'Toggle for in-app and email per type',
				'Preferences persist across sessions'
			]
		}
	],
	task_ids: [],
	planning_prompt:
		'Break this into small, independently testable tasks. Each task should be completable in one PR. Start with the backend infrastructure, then the API layer, and finally the UI components.',
	session_log: [
		'agent: Analyzing the epic requirements for real-time notifications...',
		'agent: I\'ve identified 5 tasks that build on each other. The WebSocket server and data model can be worked on in parallel, then the dispatch service ties them together.',
		'user: Can you add a task for notification preferences/settings?',
		'agent: Good idea — I\'ve added task 5 for a notification preferences page that depends on the dispatch service.',
		'agent: The dependency graph is: tasks 1 & 2 are independent → task 3 depends on both → tasks 4 & 5 depend on task 3.'
	],
	not_ready: false,
	created_at: '2025-06-01T10:00:00Z',
	updated_at: '2025-06-01T10:30:00Z'
};

// Epic in ready state — planning finished, tasks proposed, ready to confirm.
const MOCK_EPIC_READY = {
	id: 'epc_ready01',
	repo_id: 'repo_mock01',
	number: 3,
	title: 'Migrate database to PostgreSQL',
	description:
		'Migrate the application from SQLite to PostgreSQL for production readiness. Update all query files, connection handling, and deployment configuration.',
	status: 'ready',
	proposed_tasks: [
		{
			temp_id: 'tmp_r01',
			title: 'Set up PostgreSQL schema and migrations',
			description:
				'Create the initial PostgreSQL schema matching the current SQLite tables. Use golang-migrate for migration files.',
			depends_on_temp_ids: [],
			acceptance_criteria: ['All tables created', 'Migrations run cleanly']
		},
		{
			temp_id: 'tmp_r02',
			title: 'Update repository layer for pgx',
			description:
				'Replace database/sql calls with pgx/v5 in all repository implementations. Update query parameter placeholders from ? to $N.',
			depends_on_temp_ids: ['tmp_r01'],
			acceptance_criteria: ['All queries use pgx', 'Parameter placeholders updated']
		},
		{
			temp_id: 'tmp_r03',
			title: 'Add connection pooling and health checks',
			description: 'Configure pgxpool with proper connection limits and add a /health endpoint that verifies database connectivity.',
			depends_on_temp_ids: ['tmp_r02'],
			acceptance_criteria: ['Pool configured with limits', 'Health endpoint responds']
		}
	],
	task_ids: [],
	planning_prompt: 'Keep it to 3 focused tasks. We need a clean migration path.',
	session_log: [
		'agent: I\'ll break the PostgreSQL migration into 3 sequential tasks.',
		'agent: Each task builds on the previous one — schema first, then query layer, then operational concerns.',
		'user: Looks good, let\'s go with this plan.'
	],
	not_ready: false,
	created_at: '2025-06-01T08:00:00Z',
	updated_at: '2025-06-01T09:00:00Z'
};

// Epic in active state — tasks have been created and are being worked on.
const MOCK_EPIC_ACTIVE = {
	id: 'epc_active01',
	repo_id: 'repo_mock01',
	number: 4,
	title: 'Add CI/CD pipeline',
	description:
		'Set up continuous integration and deployment with GitHub Actions. Include linting, testing, building, and auto-deployment to staging.',
	status: 'active',
	proposed_tasks: [
		{
			temp_id: 'tmp_a01',
			title: 'Create CI workflow for linting and tests',
			description: 'GitHub Actions workflow that runs on every PR.',
			depends_on_temp_ids: [],
			acceptance_criteria: ['Workflow triggers on PR']
		},
		{
			temp_id: 'tmp_a02',
			title: 'Add build and Docker image workflow',
			description: 'Build the Go binary and Docker image on main branch pushes.',
			depends_on_temp_ids: ['tmp_a01'],
			acceptance_criteria: ['Image pushed to registry']
		},
		{
			temp_id: 'tmp_a03',
			title: 'Set up staging auto-deploy',
			description: 'Deploy to staging environment automatically on successful builds.',
			depends_on_temp_ids: ['tmp_a02'],
			acceptance_criteria: ['Staging updated on merge to main']
		}
	],
	task_ids: ['tsk_pending01', 'tsk_running01', 'tsk_review01'],
	planning_prompt: 'Set up a standard CI/CD pipeline with GitHub Actions.',
	session_log: [
		'agent: I\'ll set up CI/CD in 3 stages: lint/test, build, deploy.',
		'user: Perfect, confirm it.'
	],
	not_ready: false,
	created_at: '2025-05-28T12:00:00Z',
	updated_at: '2025-06-01T10:00:00Z'
};

// Epic in planning+claimed state — agent is actively planning, stop button visible.
const MOCK_EPIC_PLANNING_CLAIMED = {
	...MOCK_EPIC_PLANNING,
	id: 'epc_planningclaimed01',
	number: 5,
	claimed_at: '2025-06-01T10:10:00Z',
	last_heartbeat_at: '2025-06-01T10:15:00Z'
};

// All mock epics, used by the dashboard's epic list.
const MOCK_EPICS = [MOCK_EPIC_DRAFT, MOCK_EPIC_PLANNING, MOCK_EPIC_READY, MOCK_EPIC_ACTIVE];

// Map of epic ID to full epic object for detail pages.
const MOCK_EPIC_MAP: Record<string, typeof MOCK_EPIC_DRAFT> = {
	epc_draft01: MOCK_EPIC_DRAFT,
	epc_planning01: MOCK_EPIC_PLANNING,
	epc_planningclaimed01: MOCK_EPIC_PLANNING_CLAIMED,
	epc_ready01: MOCK_EPIC_READY,
	epc_active01: MOCK_EPIC_ACTIVE
};

// Map of epic number to full epic object for number lookups.
const MOCK_EPIC_BY_NUMBER: Record<number, typeof MOCK_EPIC_DRAFT> = {
	1: MOCK_EPIC_DRAFT,
	2: MOCK_EPIC_PLANNING,
	3: MOCK_EPIC_READY,
	4: MOCK_EPIC_ACTIVE,
	5: MOCK_EPIC_PLANNING_CLAIMED
};

// --- Mock Conversation Data ---

// Active conversation with sample messages.
const MOCK_CONVERSATION_ACTIVE = {
	id: 'cnv_active01',
	repo_id: 'repo_mock01',
	title: 'Plan authentication system',
	status: 'active',
	messages: [
		{
			role: 'user',
			content: 'I want to add JWT-based authentication to our Express API. What approach would you recommend?',
			timestamp: 1717225200
		},
		{
			role: 'assistant',
			content: 'For JWT-based authentication in your Express API, I\'d recommend the following approach:\n\n## Token Strategy\n\nUse short-lived access tokens (15 min) with longer-lived refresh tokens (7 days) stored in `httpOnly` cookies.\n\n1. **Middleware**: Create an auth middleware that validates JWTs on protected routes and extracts user context.\n2. **Password Hashing**: Use bcrypt with a cost factor of 12 for password storage.\n3. **Endpoints**:\n   - `POST /auth/login` - Issue tokens\n   - `POST /auth/register` - Create account\n   - `POST /auth/refresh` - Rotate refresh token\n   - `POST /auth/logout` - Invalidate refresh token\n\nHere\'s a sample middleware:\n\n```typescript\nimport { verify } from \'jsonwebtoken\';\n\nexport function authMiddleware(req, res, next) {\n  const token = req.cookies.access_token;\n  if (!token) return res.status(401).json({ error: \'Unauthorized\' });\n  try {\n    req.user = verify(token, process.env.JWT_SECRET);\n    next();\n  } catch {\n    res.status(401).json({ error: \'Invalid token\' });\n  }\n}\n```\n\nSee the [Express JWT docs](https://github.com/auth0/express-jwt) for more details. Would you like me to break this down into specific *implementation tasks*?',
			timestamp: 1717225260
		},
		{
			role: 'user',
			content: 'Yes, that sounds good. Also, should we add rate limiting to the auth endpoints?',
			timestamp: 1717225320
		},
		{
			role: 'assistant',
			content: 'Absolutely — rate limiting on auth endpoints is *critical* for security. I\'d recommend:\n\n- **Login endpoint**: 5 attempts per IP per 15 minutes\n- **Register endpoint**: 3 attempts per IP per hour\n- **Refresh endpoint**: 30 attempts per IP per hour\n\nYou can use `express-rate-limit` with a Redis store for distributed rate limiting, or a simple in-memory store for single-server deployments.\n\nI\'ll include rate limiting as a task when we generate the implementation plan.',
			timestamp: 1717225380
		}
	],
	created_at: '2025-06-01T09:00:00Z',
	updated_at: '2025-06-01T09:23:00Z'
};

// Conversation with a pending message (waiting for agent response).
const MOCK_CONVERSATION_PENDING = {
	id: 'cnv_pending01',
	repo_id: 'repo_mock01',
	title: 'Debug database performance',
	status: 'active',
	messages: [
		{
			role: 'user',
			content: 'Our database queries are slow on the orders table. Can you help investigate?',
			timestamp: 1717225200
		},
		{
			role: 'assistant',
			content: 'I\'d be happy to help investigate the slow queries on the orders table. Let me look at a few common causes:\n\n1. Missing indexes on frequently queried columns\n2. N+1 query patterns in the ORM\n3. Large table scans without proper WHERE clauses\n\nCan you share which specific queries are slow, or should I analyze the table schema and suggest optimizations?',
			timestamp: 1717225260
		},
		{
			role: 'user',
			content: 'The main issue is the order listing endpoint. It joins with users and products tables.',
			timestamp: 1717225320
		}
	],
	pending_message: 'The main issue is the order listing endpoint. It joins with users and products tables.',
	created_at: '2025-06-01T10:00:00Z',
	updated_at: '2025-06-01T10:05:00Z'
};

// Conversation with an epic already linked.
const MOCK_CONVERSATION_WITH_EPIC = {
	id: 'cnv_withepic01',
	repo_id: 'repo_mock01',
	title: 'Design API refactoring plan',
	status: 'active',
	messages: [
		{
			role: 'user',
			content: 'We need to refactor our API to follow REST best practices.',
			timestamp: 1717225200
		},
		{
			role: 'assistant',
			content: 'I\'ve reviewed your API structure and identified several areas for improvement. The main changes would be:\n\n1. Consistent resource naming (plural nouns)\n2. Proper HTTP status codes for each operation\n3. Pagination on list endpoints\n4. HATEOAS links in responses\n\nI\'ve generated an epic with specific tasks for each area.',
			timestamp: 1717225260
		}
	],
	epic_id: 'epc_active01',
	created_at: '2025-06-01T08:00:00Z',
	updated_at: '2025-06-01T08:30:00Z'
};

// Archived conversation.
const MOCK_CONVERSATION_ARCHIVED = {
	id: 'cnv_archived01',
	repo_id: 'repo_mock01',
	title: 'Initial project setup discussion',
	status: 'archived',
	messages: [
		{
			role: 'user',
			content: 'What tools should we use for our new project?',
			timestamp: 1717138800
		},
		{
			role: 'assistant',
			content: 'Based on your requirements, I\'d suggest SvelteKit for the frontend and Go for the backend API.',
			timestamp: 1717138860
		}
	],
	created_at: '2025-05-31T09:00:00Z',
	updated_at: '2025-05-31T09:15:00Z'
};

// Conversation with rich markdown content for visual regression testing.
const MOCK_CONVERSATION_RICH_MARKDOWN = {
	id: 'cnv_richmd01',
	repo_id: 'repo_mock01',
	title: 'Rich markdown rendering test',
	status: 'active',
	messages: [
		{
			role: 'user',
			content: 'Show me a comprehensive example of how to set up a Go HTTP server with middleware.',
			timestamp: 1717225200
		},
		{
			role: 'assistant',
			content: '## Setting Up a Go HTTP Server\n\nHere\'s a comprehensive guide to setting up a **production-ready** Go HTTP server with *middleware support*.\n\n### Key Components\n\n1. **Router**: Use `chi` or the standard `net/http` mux\n2. **Middleware**: Logging, recovery, and CORS\n3. **Graceful shutdown**: Handle `SIGINT` and `SIGTERM`\n\n### Example Code\n\nFirst, install the dependency:\n\n```bash\ngo get github.com/go-chi/chi/v5\n```\n\nThen create your server:\n\n```go\npackage main\n\nimport (\n\t"log"\n\t"net/http"\n\t"github.com/go-chi/chi/v5"\n\t"github.com/go-chi/chi/v5/middleware"\n)\n\nfunc main() {\n\tr := chi.NewRouter()\n\tr.Use(middleware.Logger)\n\tr.Use(middleware.Recoverer)\n\n\tr.Get("/health", func(w http.ResponseWriter, r *http.Request) {\n\t\tw.Write([]byte("ok"))\n\t})\n\n\tlog.Fatal(http.ListenAndServe(":8080", r))\n}\n```\n\n### Configuration Notes\n\n- Set `ReadTimeout` and `WriteTimeout` to prevent slow client attacks\n- Use `context.Context` for request-scoped values\n- The `middleware.Logger` logs each request with method, path, and duration\n\n> **Tip**: Always use `http.Server` directly instead of `http.ListenAndServe` in production for proper timeout configuration.\n\nFor more details, see the [chi documentation](https://github.com/go-chi/chi) and the [Go net/http package](https://pkg.go.dev/net/http).\n\n### Summary\n\n| Feature | Package | Notes |\n|---------|---------|-------|\n| Routing | `chi` | Lightweight, `net/http` compatible |\n| Logging | `middleware.Logger` | Built-in request logging |\n| Recovery | `middleware.Recoverer` | Panic recovery |\n\nLet me know if you want me to add `CORS` configuration or create tasks for implementing this!',
			timestamp: 1717225260
		}
	],
	created_at: '2025-06-01T11:00:00Z',
	updated_at: '2025-06-01T11:10:00Z'
};

const MOCK_CONVERSATIONS = [
	MOCK_CONVERSATION_ACTIVE,
	MOCK_CONVERSATION_PENDING,
	MOCK_CONVERSATION_WITH_EPIC,
	MOCK_CONVERSATION_ARCHIVED,
	MOCK_CONVERSATION_RICH_MARKDOWN
];

const MOCK_CONVERSATION_MAP: Record<string, typeof MOCK_CONVERSATION_ACTIVE> = {
	cnv_active01: MOCK_CONVERSATION_ACTIVE,
	cnv_pending01: MOCK_CONVERSATION_PENDING,
	cnv_withepic01: MOCK_CONVERSATION_WITH_EPIC,
	cnv_archived01: MOCK_CONVERSATION_ARCHIVED,
	cnv_richmd01: MOCK_CONVERSATION_RICH_MARKDOWN
};

// Mock agent metrics data for the agents observability page.
const MOCK_METRICS = {
	running_agents: 3,
	pending_tasks: 3,
	review_tasks: 1,
	total_tasks: 14,
	completed_tasks: 7,
	failed_tasks: 1,
	total_cost_usd: 4.52,
	active_agents: [
		{
			task_id: 'tsk_running01',
			task_number: 3,
			task_title: 'Fix database connection pooling',
			repo_id: 'repo_mock01',
			started_at: new Date(Date.now() - 12 * 60 * 1000).toISOString(), // 12 min ago
			running_for_ms: 12 * 60 * 1000,
			attempt: 1,
			cost_usd: 0.12,
			model: 'claude-sonnet-4-20250514'
		},
		{
			task_id: 'tsk_retry_running01',
			task_number: 7,
			task_title: 'Fix flaky integration tests',
			repo_id: 'repo_mock01',
			started_at: new Date(Date.now() - 5 * 60 * 1000).toISOString(), // 5 min ago
			running_for_ms: 5 * 60 * 1000,
			attempt: 2,
			cost_usd: 0.62,
			model: 'claude-sonnet-4-20250514'
		},
		{
			task_id: 'epc_planning01',
			task_number: 0,
			task_title: 'Implement user authentication system',
			repo_id: 'repo_mock01',
			started_at: new Date(Date.now() - 3 * 60 * 1000).toISOString(), // 3 min ago
			running_for_ms: 3 * 60 * 1000,
			attempt: 0,
			cost_usd: 0,
			model: 'claude-sonnet-4-20250514',
			is_planning: true,
			epic_id: 'epc_planning01',
			epic_title: 'Implement user authentication system'
		}
	],
	workers: [
		{
			worker_id: 'wrk_abc12345',
			max_concurrent_tasks: 4,
			active_tasks: 2,
			connected_at: new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString(), // 3 hours ago
			last_poll_at: new Date(Date.now() - 5 * 1000).toISOString(), // 5 seconds ago
			uptime_ms: 3 * 60 * 60 * 1000,
			polling: false
		},
		{
			worker_id: 'wrk_def67890',
			max_concurrent_tasks: 2,
			active_tasks: 0,
			connected_at: new Date(Date.now() - 45 * 60 * 1000).toISOString(), // 45 min ago
			last_poll_at: new Date(Date.now() - 2 * 1000).toISOString(), // 2 seconds ago
			uptime_ms: 45 * 60 * 1000,
			polling: true
		}
	],
	recent_completions: [
		{
			task_id: 'tsk_review01',
			task_number: 4,
			task_title: 'Add dark mode support',
			repo_id: 'repo_mock01',
			status: 'merged',
			duration_ms: 180000,
			cost_usd: 0.45,
			attempt: 1,
			finished_at: new Date(Date.now() - 30 * 60 * 1000).toISOString() // 30 min ago
		},
		{
			task_id: 'tsk_merged01',
			task_number: 5,
			task_title: 'Update API documentation',
			repo_id: 'repo_mock01',
			status: 'merged',
			duration_ms: 120000,
			cost_usd: 0.30,
			attempt: 1,
			finished_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString() // 2 hours ago
		},
		{
			task_id: 'tsk_failed01',
			task_number: 6,
			task_title: 'Migrate to new ORM',
			repo_id: 'repo_mock01',
			status: 'failed',
			duration_ms: 300000,
			cost_usd: 0.85,
			attempt: 2,
			finished_at: new Date(Date.now() - 4 * 60 * 60 * 1000).toISOString() // 4 hours ago
		},
		{
			task_id: 'tsk_closed01',
			task_number: 9,
			task_title: 'Refactor legacy auth module',
			repo_id: 'repo_mock01',
			status: 'closed',
			duration_ms: 95000,
			cost_usd: 0.18,
			attempt: 1,
			finished_at: new Date(Date.now() - 6 * 60 * 60 * 1000).toISOString() // 6 hours ago
		}
	]
};

// A realistic unified diff for the "dark mode support" review task.
// This gives the DiffViewer component something meaningful to render in screenshots.
const MOCK_DIFF = `diff --git a/tailwind.config.js b/tailwind.config.js
index 3b8a1c2..f7d9e4a 100644
--- a/tailwind.config.js
+++ b/tailwind.config.js
@@ -1,7 +1,7 @@
 /** @type {import('tailwindcss').Config} */
 export default {
   content: ['./src/**/*.{html,js,svelte,ts}'],
-  darkMode: 'media',
+  darkMode: 'class',
   theme: {
     extend: {
       colors: {
diff --git a/src/lib/stores/theme.ts b/src/lib/stores/theme.ts
new file mode 100644
index 0000000..a4c8f29
--- /dev/null
+++ b/src/lib/stores/theme.ts
@@ -0,0 +1,29 @@
+import { writable } from 'svelte/store';
+import { browser } from '$app/environment';
+
+export type ThemePreference = 'light' | 'dark' | 'system';
+
+function getInitialTheme(): ThemePreference {
+  if (browser) {
+    const stored = localStorage.getItem('theme-preference');
+    if (stored === 'light' || stored === 'dark' || stored === 'system') {
+      return stored;
+    }
+  }
+  return 'system';
+}
+
+export const themePreference = writable<ThemePreference>(getInitialTheme());
+
+export function applyTheme(pref: ThemePreference) {
+  if (!browser) return;
+  localStorage.setItem('theme-preference', pref);
+
+  const isDark =
+    pref === 'dark' ||
+    (pref === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
+
+  document.documentElement.classList.toggle('dark', isDark);
+}
+
+// Re-apply when OS preference changes
+themePreference.subscribe(applyTheme);
diff --git a/src/lib/components/Header.svelte b/src/lib/components/Header.svelte
index 8e2f1a0..b3c4d72 100644
--- a/src/lib/components/Header.svelte
+++ b/src/lib/components/Header.svelte
@@ -1,8 +1,12 @@
 <script lang="ts">
   import { page } from '$app/stores';
+  import { themePreference, type ThemePreference } from '$lib/stores/theme';
+  import Sun from 'lucide-svelte/icons/sun';
+  import Moon from 'lucide-svelte/icons/moon';
+  import Monitor from 'lucide-svelte/icons/monitor';

-  export let title = 'My App';
-  let menuOpen = false;
+  let { title = 'My App' }: { title?: string } = $props();
+  let menuOpen = $state(false);
 </script>

 <header class="border-b bg-background">
@@ -12,5 +16,22 @@
       <a href="/settings" class="text-sm text-muted-foreground hover:text-foreground">
         Settings
       </a>
+      <div class="flex items-center gap-1 ml-2 rounded-lg border p-0.5">
+        <button
+          class="p-1.5 rounded {$themePreference === 'light' ? 'bg-muted' : ''}"
+          onclick={() => themePreference.set('light')}
+          aria-label="Light mode"
+        >
+          <Sun class="w-4 h-4" />
+        </button>
+        <button
+          class="p-1.5 rounded {$themePreference === 'dark' ? 'bg-muted' : ''}"
+          onclick={() => themePreference.set('dark')}
+          aria-label="Dark mode"
+        >
+          <Moon class="w-4 h-4" />
+        </button>
+      </div>
     </nav>
   </div>
`;

// Intercept all API calls so the UI renders with mock data instead of hitting a real server.
// Routes are registered most-specific first because Playwright matches in FIFO order.
// The `repoOverride` parameter allows individual tests to swap the mock repo (e.g. to
// simulate scanning or needs_setup states) without affecting other tests.
async function setupMockAPI(
	page: import('@playwright/test').Page,
	repoOverride?: typeof MOCK_REPO
) {
	const activeRepo = repoOverride ?? MOCK_REPO;
	// GitHub token status - report as configured so the UI shows the dashboard.
	await page.route('**/api/v1/settings/github-token', (route) =>
		route.fulfill({ json: { data: { configured: true, fine_grained: true } } })
	);

	// Default model
	await page.route('**/api/v1/settings/default-model', (route) =>
		route.fulfill({ json: { data: { model: 'claude-sonnet-4-20250514', configured: true } } })
	);

	// Available models list
	await page.route('**/api/v1/settings/models', (route) =>
		route.fulfill({ json: { data: [
			{ value: 'haiku', label: 'Haiku' },
			{ value: 'sonnet', label: 'Sonnet' },
			{ value: 'opus', label: 'Opus' }
		] } })
	);

	// Agent metrics
	await page.route('**/api/v1/metrics', (route) =>
		route.fulfill({ json: { data: MOCK_METRICS } })
	);

	// Repo setup endpoints (must be before generic /repos/* routes)
	await page.route('**/api/v1/repos/*/setup/rescan', (route) =>
		route.fulfill({ json: { data: { ...activeRepo, setup_status: 'scanning' } } })
	);
	await page.route('**/api/v1/repos/*/setup/skip', (route) =>
		route.fulfill({ json: { data: { ...activeRepo, setup_status: 'ready', setup_completed_at: new Date().toISOString() } } })
	);
	await page.route('**/api/v1/repos/*/setup/submit', (route) =>
		route.fulfill({ json: { data: { ...activeRepo, setup_status: 'configuring' } } })
	);
	await page.route('**/api/v1/repos/*/setup/confirm', (route) =>
		route.fulfill({ json: { data: { ...activeRepo, setup_status: 'ready', setup_completed_at: new Date().toISOString() } } })
	);
	await page.route('**/api/v1/repos/*/setup', (route) => {
		if (route.request().method() === 'PATCH') {
			return route.fulfill({ json: { data: { ...activeRepo, setup_status: 'ready', setup_completed_at: new Date().toISOString() } } });
		}
		return route.fulfill({ json: { data: activeRepo } });
	});

	// Repos list
	await page.route('**/api/v1/repos', (route) => {
		if (route.request().method() === 'GET') {
			return route.fulfill({ json: { data: [activeRepo] } });
		}
		return route.fulfill({ json: { data: activeRepo } });
	});

	// SSE events endpoint - send an init event with mock tasks then keep connection open.
	await page.route('**/api/v1/events**', (route) => {
		const body = `event: init\ndata: ${JSON.stringify(MOCK_TASKS)}\n\n`;
		return route.fulfill({
			status: 200,
			headers: {
				'Content-Type': 'text/event-stream',
				'Cache-Control': 'no-cache',
				Connection: 'keep-alive'
			},
			body
		});
	});

	// Task by number lookup (must be before generic /repos/* catch-all).
	// Note: Playwright glob does NOT support [0-9] character classes — use a
	// URL predicate instead so that numeric task-number segments match correctly.
	await page.route(
		(url) => /\/api\/v1\/repos\/[^/]+\/tasks\/\d+(\/|\?|$)/.test(url.pathname),
		(route) => {
			const url = route.request().url();
			const numberStr = url.split('/tasks/')[1]?.split(/[/?]/)[0];
			const number = Number(numberStr);
			const task = MOCK_TASKS.find((t) => t.number === number);
			if (task) {
				return route.fulfill({ json: { data: task } });
			}
			return route.fulfill({ status: 404, json: { error: { message: 'not found' } } });
		}
	);

	// Task diff (must be before generic /tasks/* route).
	await page.route('**/api/v1/tasks/*/diff', (route) =>
		route.fulfill({ json: { data: { diff: MOCK_DIFF } } })
	);

	// Task checks (must be before generic /tasks/* route).
	await page.route('**/api/v1/tasks/*/checks', (route) =>
		route.fulfill({
			json: { data: {
				status: 'success',
				summary: '3/3 checks passed',
				checks: [
					{
						name: 'build',
						status: 'completed',
						conclusion: 'success',
						url: 'https://github.com'
					},
					{
						name: 'lint',
						status: 'completed',
						conclusion: 'success',
						url: 'https://github.com'
					},
					{
						name: 'test',
						status: 'completed',
						conclusion: 'success',
						url: 'https://github.com'
					}
				]
			} }
		})
	);

	// Task logs SSE (must be before generic /tasks/* route).
	// Sends per-attempt logs_appended events followed by logs_done so the UI
	// renders them in the terminal with full syntax highlighting.
	await page.route('**/api/v1/tasks/*/logs', (route) => {
		const url = route.request().url();
		const taskId = url.split('/tasks/')[1]?.split('/')[0];
		const logsByAttempt = taskId ? MOCK_TASK_LOGS[taskId] : undefined;

		let body = '';
		if (logsByAttempt) {
			for (const [attempt, lines] of Object.entries(logsByAttempt)) {
				body += `event: logs_appended\ndata: ${JSON.stringify({ attempt: Number(attempt), logs: lines })}\n\n`;
			}
		}
		body += 'event: logs_done\ndata: {}\n\n';

		return route.fulfill({
			status: 200,
			headers: {
				'Content-Type': 'text/event-stream',
				'Cache-Control': 'no-cache',
				Connection: 'keep-alive'
			},
			body
		});
	});

	// Individual task detail (generic catch-all for /tasks/*).
	await page.route('**/api/v1/tasks/*', (route) => {
		const url = route.request().url();
		const taskId = url.split('/tasks/')[1]?.split('/')[0]?.split('?')[0];
		const task = MOCK_TASKS.find((t) => t.id === taskId);
		if (task) {
			return route.fulfill({ json: { data: task } });
		}
		return route.fulfill({ status: 404, json: { error: { message: 'not found' } } });
	});

	// --- Conversation API mocks ---

	// List conversations for a repo (must be before generic /repos/* catch-all).
	await page.route('**/api/v1/repos/*/conversations', (route) => {
		if (route.request().method() === 'POST') {
			return route.fulfill({ json: { data: MOCK_CONVERSATION_ACTIVE } });
		}
		return route.fulfill({ json: { data: MOCK_CONVERSATIONS } });
	});

	// Conversation sub-resource routes (must be before the generic /conversations/* catch-all).
	await page.route('**/api/v1/conversations/*/messages', (route) =>
		route.fulfill({ json: { data: MOCK_CONVERSATION_PENDING } })
	);
	await page.route('**/api/v1/conversations/*/archive', (route) =>
		route.fulfill({ json: { data: { ...MOCK_CONVERSATION_ACTIVE, status: 'archived' } } })
	);
	await page.route('**/api/v1/conversations/*/generate-tasks', (route) =>
		route.fulfill({ json: { data: { epic_id: 'epc_active01' } } })
	);

	// Individual conversation detail (generic catch-all for /conversations/*).
	await page.route('**/api/v1/conversations/*', (route) => {
		const url = route.request().url();
		const convId = url.split('/conversations/')[1]?.split('/')[0]?.split('?')[0];
		const conv = convId ? MOCK_CONVERSATION_MAP[convId] : undefined;
		if (conv) {
			return route.fulfill({ json: { data: conv } });
		}
		return route.fulfill({ status: 404, json: { error: { message: 'not found' } } });
	});

	// --- Epic API mocks ---

	// Epic number lookup (must be before generic /repos/*/epics catch-all).
	// Note: Playwright glob does NOT support [0-9] character classes — use a
	// URL predicate instead so that numeric epic-number segments match correctly.
	await page.route(
		(url) => /\/api\/v1\/repos\/[^/]+\/epics\/\d+(\/|\?|$)/.test(url.pathname),
		(route) => {
			const url = route.request().url();
			const num = parseInt(url.split('/epics/')[1]?.split(/[/?]/)[0] ?? '0');
			const epic = MOCK_EPIC_BY_NUMBER[num];
			if (epic) {
				return route.fulfill({ json: { data: epic } });
			}
			return route.fulfill({ status: 404, json: { error: { message: 'not found' } } });
		}
	);

	// List epics for a repo (must be before generic /repos/* catch-all).
	await page.route('**/api/v1/repos/*/epics', (route) => {
		if (route.request().method() === 'POST') {
			// Create epic — return a new draft epic.
			return route.fulfill({ json: { data: MOCK_EPIC_DRAFT } });
		}
		return route.fulfill({ json: { data: MOCK_EPICS } });
	});

	// Epic tasks (must be before the generic /epics/* catch-all).
	await page.route('**/api/v1/epics/*/tasks', (route) => {
		const url = route.request().url();
		const epicId = url.split('/epics/')[1]?.split('/')[0];
		const epic = epicId ? MOCK_EPIC_MAP[epicId] : undefined;
		if (epic && epic.task_ids.length > 0) {
			const tasks = epic.task_ids.map((tid: string) => {
				const t = MOCK_TASKS.find((mt) => mt.id === tid);
				return t ? { id: t.id, number: t.number, title: t.title, status: t.status } : { id: tid, number: 0, title: tid, status: 'pending' };
			});
			return route.fulfill({ json: { data: tasks } });
		}
		return route.fulfill({ json: { data: [] } });
	});

	// Epic sub-resource routes (must be before the generic /epics/* catch-all).
	await page.route('**/api/v1/epics/*/plan', (route) =>
		route.fulfill({ json: { data: MOCK_EPIC_PLANNING } })
	);
	await page.route('**/api/v1/epics/*/proposed-tasks', (route) =>
		route.fulfill({ json: { data: MOCK_EPIC_READY } })
	);
	await page.route('**/api/v1/epics/*/session-message', (route) =>
		route.fulfill({ json: { data: MOCK_EPIC_PLANNING } })
	);
	await page.route('**/api/v1/epics/*/confirm', (route) =>
		route.fulfill({ json: { data: MOCK_EPIC_ACTIVE } })
	);
	await page.route('**/api/v1/epics/*/close', (route) =>
		route.fulfill({ json: { data: { ...MOCK_EPIC_DRAFT, status: 'closed' } } })
	);
	await page.route('**/api/v1/epics/*/stop', (route) =>
		route.fulfill({ json: { data: { ...MOCK_EPIC_PLANNING_CLAIMED, status: 'draft', claimed_at: null } } })
	);

	// Individual epic detail (generic catch-all for /epics/*).
	await page.route('**/api/v1/epics/*', (route) => {
		const url = route.request().url();
		const epicId = url.split('/epics/')[1]?.split('/')[0]?.split('?')[0];
		const epic = epicId ? MOCK_EPIC_MAP[epicId] : undefined;
		if (epic) {
			return route.fulfill({ json: { data: epic } });
		}
		return route.fulfill({ status: 404, json: { error: { message: 'not found' } } });
	});
}

test.describe('UI Screenshots', () => {
	test('dashboard', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/');

		// Wait for tasks to render.
		await page.waitForSelector('[data-testid="task-card"], .task-card, [class*="Card"]', {
			timeout: 5000
		}).catch(() => {
			// Fallback: wait for any content to load.
		});

		// Give the UI a moment to settle after SSE data loads.
		await page.waitForTimeout(1500);

		await page.screenshot({
			path: `screenshots/dashboard-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	// --- Repo Setup Screenshots ---

	test('dashboard - repo scanning banner', async ({ page }, testInfo) => {
		await setupMockAPI(page, MOCK_REPO_SCANNING);
		await page.goto('/');

		// Wait for the scanning banner to render (no kanban columns visible).
		await page.waitForSelector('text=Scanning repository', { timeout: 5000 });
		await page.waitForTimeout(1500);

		await page.screenshot({
			path: `screenshots/repo-setup-scanning-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('dashboard - repo needs setup banner', async ({ page }, testInfo) => {
		await setupMockAPI(page, MOCK_REPO_NEEDS_SETUP);
		await page.goto('/');

		// Wait for the needs-setup banner to render (no kanban columns visible).
		await page.waitForSelector('text=Repository needs configuration', { timeout: 5000 });
		await page.waitForTimeout(1500);

		// Expand the "Scan Results" section to show the RepoSummary
		const scanResultsBtn = page.getByText('Scan Results');
		if (await scanResultsBtn.isVisible()) {
			await scanResultsBtn.click();
			await page.waitForTimeout(500);
		}

		await page.screenshot({
			path: `screenshots/repo-setup-needs-setup-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('repo setup wizard dialog', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 1280, height: 1600 });
		await setupMockAPI(page, MOCK_REPO_NEEDS_SETUP);
		await page.goto('/');

		// Wait for the needs-setup banner to render.
		await page.waitForSelector('text=Repository needs configuration', { timeout: 5000 });
		await page.waitForTimeout(1500);

		// Click the "Configure" button in the needs_setup banner to open the wizard
		const configureBtn = page.getByRole('button', { name: /configure/i });
		await configureBtn.click();

		// Wait for dialog to appear and settle.
		await page.waitForTimeout(1000);

		// Screenshot the dialog element directly to capture its full content.
		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/repo-setup-wizard-${testInfo.project.name}.png`
		});
	});

	test('repo setup wizard dialog - empty tech stack', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 1280, height: 1600 });
		await setupMockAPI(page, MOCK_REPO_NEEDS_SETUP_EMPTY);
		await page.goto('/');

		// Wait for the needs-setup banner to render.
		await page.waitForSelector('text=Repository needs configuration', { timeout: 5000 });
		await page.waitForTimeout(1500);

		// Click the "Configure" button in the needs_setup banner to open the wizard
		const configureBtn = page.getByRole('button', { name: /configure/i });
		await configureBtn.click();

		// Wait for dialog to appear and settle.
		await page.waitForTimeout(1000);

		// Screenshot the dialog element directly to capture its full content.
		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/repo-setup-wizard-empty-stack-${testInfo.project.name}.png`
		});
	});

	test('dashboard - repo configuring banner', async ({ page }, testInfo) => {
		await setupMockAPI(page, MOCK_REPO_CONFIGURING);
		await page.goto('/');

		// Wait for the configuring banner to render.
		await page.waitForSelector('text=AI reviewing configuration', { timeout: 5000 });
		await page.waitForTimeout(1500);

		await page.screenshot({
			path: `screenshots/repo-setup-configuring-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('dashboard - repo pending setup banner', async ({ page }, testInfo) => {
		await setupMockAPI(page, MOCK_REPO_PENDING);
		await page.goto('/');

		// Wait for the pending setup banner to render (no kanban columns visible).
		await page.waitForSelector('text=Repository setup required', { timeout: 5000 });
		await page.waitForTimeout(1500);

		await page.screenshot({
			path: `screenshots/repo-setup-pending-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('repo settings dialog', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 1280, height: 900 });
		await setupMockAPI(page, MOCK_REPO_NEEDS_SETUP);
		await page.goto('/');

		await page.waitForTimeout(1500);

		// Click the "Repo Settings" sidebar link
		const repoSettingsBtn = page.getByRole('button', { name: /repo settings/i });
		await repoSettingsBtn.click();

		await page.waitForTimeout(1000);

		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/repo-settings-dialog-${testInfo.project.name}.png`
		});
	});

	test('task detail - review', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/acme/webapp/tasks/4`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-detail-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - pr view', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/acme/webapp/tasks/4/pr`);

		// Wait for the diff to auto-expand and render (file headers appear once loaded).
		await page.waitForSelector('table', { timeout: 15000 });
		await page.waitForTimeout(500);

		await page.screenshot({
			path: `screenshots/task-pr-view-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - running', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/acme/webapp/tasks/3`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-running-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - retry running', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/acme/webapp/tasks/7`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-retry-running-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - retry pending', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/acme/webapp/tasks/8`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-retry-pending-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - not ready', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/acme/webapp/tasks/2`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-not-ready-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('edit task dialog', async ({ page }, testInfo) => {
		// Use a tall viewport so the dialog's max-h-[90vh] doesn't clip content.
		await page.setViewportSize({ width: 1280, height: 1600 });
		await setupMockAPI(page);
		await page.goto('/acme/webapp/tasks/1');

		// Wait for the Edit button to be visible, confirming the task has loaded.
		const editButton = page.getByRole('button', { name: /edit/i });
		await editButton.waitFor({ timeout: 15000 });
		await editButton.click();

		// Wait for dialog to appear and settle.
		await page.waitForTimeout(1000);

		// Screenshot the dialog element directly to capture its full content.
		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/edit-task-dialog-${testInfo.project.name}.png`
		});
	});

	test('create task dialog', async ({ page }, testInfo) => {
		// Use a tall viewport so the dialog's max-h-[90vh] doesn't clip content.
		await page.setViewportSize({ width: 1280, height: 1600 });
		await setupMockAPI(page);
		await page.goto('/');

		// Wait for dashboard to load.
		await page.waitForSelector('[data-testid="task-card"], .task-card, [class*="Card"]', {
			timeout: 5000
		}).catch(() => {});
		await page.waitForTimeout(1000);

		// Click the "New Task" button to open the dialog.
		const createButton = page.getByRole('button', { name: /new task/i });
		await createButton.click();

		// Wait for dialog to appear and settle.
		await page.waitForTimeout(1000);

		// Screenshot the dialog element directly to capture its full content.
		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/create-task-dialog-${testInfo.project.name}.png`
		});
	});

	// --- Metrics Screenshots ---

	test('metrics dashboard', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/agents');

		// Wait for metrics to load - ensure we see actual data, not loading state.
		// Wait for the Tasks heading to appear
		await page.waitForSelector('h2:has-text("Tasks")', { timeout: 5000 });
		// Wait for specific numeric values to be rendered in the metric cards
		await page.waitForSelector('.text-2xl.font-bold', { timeout: 5000 });
		// Wait for Connected Workers section to ensure full page load
		await page.waitForSelector('h2:has-text("Connected Workers")', { timeout: 5000 });
		// Additional wait to ensure all components have rendered
		await page.waitForTimeout(1500);

		await page.screenshot({
			path: `screenshots/metrics-dashboard-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	// --- Epic Screenshots ---

	test('create epic dialog', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 1280, height: 1600 });
		await setupMockAPI(page);
		await page.goto('/epics');

		// Wait for epics page to load.
		await page.waitForTimeout(2000);

		// Click the "New Epic" button to open the dialog.
		const createButton = page.getByRole('button', { name: /new epic/i });
		await createButton.click();

		// Wait for dialog to appear and settle.
		await page.waitForTimeout(1000);

		// Screenshot the dialog element directly.
		const dialog = page.locator('[role="dialog"]');
		await dialog.screenshot({
			path: `screenshots/create-epic-dialog-${testInfo.project.name}.png`
		});
	});

	test('epic detail - draft', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/acme/webapp/epics/1');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-draft-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - planning with proposed tasks', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/acme/webapp/epics/2');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-planning-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - planning claimed with stop button', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/acme/webapp/epics/5');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-planning-claimed-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - ready with confirm section', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/acme/webapp/epics/3');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-ready-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - active with created tasks', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/acme/webapp/epics/4');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-active-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - redirect from old ID route', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/epics/epc_draft01');

		await page.waitForTimeout(2000);

		// Should have redirected to the number-based URL.
		await page.screenshot({
			path: `screenshots/epic-redirect-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	// --- Conversation Screenshots ---

	test('conversation list page', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/conversations');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/conversation-list-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('conversation list page - empty state', async ({ page }, testInfo) => {
		await setupMockAPI(page);

		// Override conversations route to return empty list.
		await page.route('**/api/v1/repos/*/conversations', (route) =>
			route.fulfill({ json: { data: [] } })
		);

		await page.goto('/conversations');
		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/conversation-list-empty-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('conversation detail - message history', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/conversations/cnv_active01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/conversation-detail-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('conversation detail - pending response', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/conversations/cnv_pending01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/conversation-pending-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('conversation detail - with linked epic', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/conversations/cnv_withepic01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/conversation-with-epic-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('conversation detail - rich markdown message', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/conversations/cnv_richmd01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/conversation-rich-markdown-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	// --- Mobile Bottom Bar Screenshots ---

	test('mobile bottom bar - tasks page', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 375, height: 812 });
		await setupMockAPI(page);
		await page.goto('/');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/mobile-bottom-bar-tasks-${testInfo.project.name}.png`,
			fullPage: false
		});
	});

	test('mobile bottom bar - epics page', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 375, height: 812 });
		await setupMockAPI(page);
		await page.goto('/epics');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/mobile-bottom-bar-epics-${testInfo.project.name}.png`,
			fullPage: false
		});
	});

});

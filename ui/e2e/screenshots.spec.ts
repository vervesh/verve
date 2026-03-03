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
		repo_id: 'repo_mock01',
		title: 'Refactor payment processing module',
		description: 'Break up the monolithic payment handler into smaller services',
		status: 'pending',
		logs: [],
		attempt: 1,
		max_attempts: 3,
		acceptance_criteria: ['Payment tests pass', 'No regressions'],
		consecutive_failures: 0,
		cost_usd: 0,
		skip_pr: false,
		ready: false,
		created_at: '2025-06-01T08:00:00Z',
		updated_at: '2025-06-01T08:00:00Z'
	},
	{
		id: 'tsk_running01',
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
	title: 'Implement user authentication system',
	description:
		'Build a complete authentication system with JWT tokens, login/signup pages, password reset flow, and role-based access control. Should integrate with the existing Express API and use PostgreSQL for user storage.',
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
	title: 'Add real-time notifications',
	description:
		'Implement a real-time notification system using WebSockets. Users should receive notifications for task completions, PR reviews, and system alerts. Include a notification bell in the header with an unread count badge.',
	status: 'planning',
	proposed_tasks: [
		{
			temp_id: 'tmp_01',
			title: 'Set up WebSocket server infrastructure',
			description:
				'Create a WebSocket server using ws library integrated with the Express app. Handle connection lifecycle, authentication via JWT, and heartbeat pings.',
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

// All mock epics, used by the dashboard's epic list.
const MOCK_EPICS = [MOCK_EPIC_DRAFT, MOCK_EPIC_PLANNING, MOCK_EPIC_READY, MOCK_EPIC_ACTIVE];

// Map of epic ID to full epic object for detail pages.
const MOCK_EPIC_MAP: Record<string, typeof MOCK_EPIC_DRAFT> = {
	epc_draft01: MOCK_EPIC_DRAFT,
	epc_planning01: MOCK_EPIC_PLANNING,
	epc_ready01: MOCK_EPIC_READY,
	epc_active01: MOCK_EPIC_ACTIVE
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
	await page.route('**/api/v1/repos/*/setup/expectations', (route) =>
		route.fulfill({ json: { data: { ...activeRepo, setup_status: 'ready', setup_completed_at: new Date().toISOString() } } })
	);
	await page.route('**/api/v1/repos/*/setup/rescan', (route) =>
		route.fulfill({ json: { data: { ...activeRepo, setup_status: 'scanning' } } })
	);
	await page.route('**/api/v1/repos/*/setup', (route) =>
		route.fulfill({ json: { data: activeRepo } })
	);

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

	// --- Epic API mocks ---

	// List epics for a repo (must be before generic /repos/* catch-all).
	await page.route('**/api/v1/repos/*/epics', (route) => {
		if (route.request().method() === 'POST') {
			// Create epic — return a new draft epic.
			return route.fulfill({ json: { data: MOCK_EPIC_DRAFT } });
		}
		return route.fulfill({ json: { data: MOCK_EPICS } });
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

		// Wait for tasks to render.
		await page.waitForSelector('[data-testid="task-card"], .task-card, [class*="Card"]', {
			timeout: 5000
		}).catch(() => {});

		await page.waitForTimeout(1500);

		await page.screenshot({
			path: `screenshots/repo-setup-scanning-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('dashboard - repo needs setup banner', async ({ page }, testInfo) => {
		await setupMockAPI(page, MOCK_REPO_NEEDS_SETUP);
		await page.goto('/');

		// Wait for tasks to render.
		await page.waitForSelector('[data-testid="task-card"], .task-card, [class*="Card"]', {
			timeout: 5000
		}).catch(() => {});

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

		// Wait for tasks to render.
		await page.waitForSelector('[data-testid="task-card"], .task-card, [class*="Card"]', {
			timeout: 5000
		}).catch(() => {});
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

	test('task detail - review', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_review01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-detail-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - pr view', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_review01/pr`);

		await page.waitForTimeout(2000);

		// Wait for the diff to auto-expand and render (file headers appear once loaded).
		await page.waitForSelector('table', { timeout: 5000 });
		await page.waitForTimeout(500);

		await page.screenshot({
			path: `screenshots/task-pr-view-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - running', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_running01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-running-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - retry running', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_retry_running01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-retry-running-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - retry pending', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_retry_pending01`);

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/task-retry-pending-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('task detail - not ready', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto(`/tasks/tsk_notready01`);

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
		await page.goto('/tasks/tsk_pending01');

		// Wait for task detail to load.
		await page.waitForTimeout(2000);

		// Click the "Edit" button to open the dialog.
		const editButton = page.getByRole('button', { name: /edit/i });
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
		await page.goto('/epics/epc_draft01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-draft-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - planning with proposed tasks', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/epics/epc_planning01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-planning-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - ready with confirm section', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/epics/epc_ready01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-ready-${testInfo.project.name}.png`,
			fullPage: true
		});
	});

	test('epic detail - active with created tasks', async ({ page }, testInfo) => {
		await setupMockAPI(page);
		await page.goto('/epics/epc_active01');

		await page.waitForTimeout(2000);

		await page.screenshot({
			path: `screenshots/epic-active-${testInfo.project.name}.png`,
			fullPage: true
		});
	});
});

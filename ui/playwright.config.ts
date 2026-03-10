import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	outputDir: './e2e/results',
	fullyParallel: false,
	forbidOnly: true,
	retries: 0,
	workers: 1,
	reporter: 'list',
	use: {
		baseURL: 'http://localhost:4173',
		trace: 'off'
	},
	projects: [
		{
			name: 'desktop',
			use: { ...devices['Desktop Chrome'], viewport: { width: 1280, height: 900 } }
		},
		{
			name: 'mobile',
			use: { ...devices['iPhone 14'], viewport: { width: 393, height: 852 } }
		}
	],
	webServer: {
		command: 'pnpm preview --port 4173',
		port: 4173,
		reuseExistingServer: false
	}
});

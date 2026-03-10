<script lang="ts">
	import { client } from '$lib/api-client';
	import {
		ChevronDown,
		Loader2,
		AlertTriangle,
		FileText,
		ExternalLink,
		ChevronRight,
		FilePlus,
		FileX,
		FileEdit,
		List
	} from 'lucide-svelte';

	interface DiffLine {
		type: 'addition' | 'deletion' | 'context' | 'hunk-header';
		content: string;
		oldLineNum: number | null;
		newLineNum: number | null;
	}

	interface DiffHunk {
		header: string;
		lines: DiffLine[];
	}

	interface DiffFile {
		oldPath: string;
		newPath: string;
		hunks: DiffHunk[];
		expanded: boolean;
		additions: number;
		deletions: number;
	}

	interface ParsedDiff {
		files: DiffFile[];
		totalLines: number;
	}

	let { taskId, hasPR, prUrl = '', autoExpand = false }: { taskId: string; hasPR: boolean; prUrl?: string; autoExpand?: boolean } = $props();

	let expanded = $state(false);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let parsedDiff = $state<ParsedDiff | null>(null);
	let fetched = $state(false);
	let showFileNav = $state(true);

	function parseDiff(raw: string): ParsedDiff {
		if (!raw || !raw.trim()) {
			return { files: [], totalLines: 0 };
		}

		const files: DiffFile[] = [];
		let totalLines = 0;

		// Split on file boundaries
		const fileSections = raw.split(/^diff --git /m).filter((s) => s.trim());

		for (const section of fileSections) {
			const lines = section.split('\n');
			// First line has the a/... b/... paths
			const headerLine = lines[0];
			const pathMatch = headerLine.match(/a\/(.+?)\s+b\/(.+)/);
			const oldPath = pathMatch ? pathMatch[1] : 'unknown';
			const newPath = pathMatch ? pathMatch[2] : 'unknown';

			const hunks: DiffHunk[] = [];
			let currentHunk: DiffHunk | null = null;
			let oldLine = 0;
			let newLine = 0;
			let fileAdditions = 0;
			let fileDeletions = 0;

			for (let i = 1; i < lines.length; i++) {
				const line = lines[i];

				// Hunk header
				const hunkMatch = line.match(/^@@\s+-(\d+)(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@(.*)/);
				if (hunkMatch) {
					oldLine = parseInt(hunkMatch[1], 10);
					newLine = parseInt(hunkMatch[2], 10);
					currentHunk = {
						header: line,
						lines: [
							{
								type: 'hunk-header',
								content: line,
								oldLineNum: null,
								newLineNum: null
							}
						]
					};
					hunks.push(currentHunk);
					totalLines++;
					continue;
				}

				// Skip file metadata lines (index, ---, +++)
				if (
					line.startsWith('index ') ||
					line.startsWith('--- ') ||
					line.startsWith('+++ ') ||
					line.startsWith('old mode') ||
					line.startsWith('new mode') ||
					line.startsWith('new file mode') ||
					line.startsWith('deleted file mode') ||
					line.startsWith('similarity index') ||
					line.startsWith('rename from') ||
					line.startsWith('rename to') ||
					line.startsWith('Binary files')
				) {
					continue;
				}

				if (!currentHunk) continue;

				if (line.startsWith('+')) {
					currentHunk.lines.push({
						type: 'addition',
						content: line.substring(1),
						oldLineNum: null,
						newLineNum: newLine
					});
					newLine++;
					totalLines++;
					fileAdditions++;
				} else if (line.startsWith('-')) {
					currentHunk.lines.push({
						type: 'deletion',
						content: line.substring(1),
						oldLineNum: oldLine,
						newLineNum: null
					});
					oldLine++;
					totalLines++;
					fileDeletions++;
				} else if (line.startsWith(' ') || line === '') {
					// Context line (or empty trailing line)
					if (line === '' && i === lines.length - 1) continue;
					currentHunk.lines.push({
						type: 'context',
						content: line.startsWith(' ') ? line.substring(1) : line,
						oldLineNum: oldLine,
						newLineNum: newLine
					});
					oldLine++;
					newLine++;
					totalLines++;
				} else if (line.startsWith('\\')) {
					// "\ No newline at end of file" - skip
					continue;
				}
			}

			files.push({
				oldPath,
				newPath,
				hunks,
				expanded: true,
				additions: fileAdditions,
				deletions: fileDeletions
			});
		}

		return { files, totalLines };
	}

	async function fetchDiff() {
		if (fetched) return;
		loading = true;
		error = null;
		try {
			const result = await client.getTaskDiff(taskId);
			parsedDiff = parseDiff(result.diff);
			fetched = true;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to fetch diff';
		} finally {
			loading = false;
		}
	}

	function toggleExpanded() {
		expanded = !expanded;
		if (expanded && !fetched && !loading) {
			fetchDiff();
		}
	}

	function toggleFile(index: number) {
		if (parsedDiff) {
			parsedDiff.files[index].expanded = !parsedDiff.files[index].expanded;
		}
	}

	function scrollToFile(index: number) {
		const el = document.getElementById(`diff-file-${index}`);
		if (el) {
			el.scrollIntoView({ behavior: 'smooth', block: 'start' });
		}
		// Ensure the file is expanded
		if (parsedDiff && !parsedDiff.files[index].expanded) {
			parsedDiff.files[index].expanded = true;
		}
	}

	function getFileName(filePath: string): string {
		const parts = filePath.split('/');
		return parts[parts.length - 1];
	}

	function getFileDir(filePath: string): string {
		const parts = filePath.split('/');
		if (parts.length <= 1) return '';
		return parts.slice(0, -1).join('/') + '/';
	}

	function getFileStatus(file: DiffFile): 'added' | 'deleted' | 'modified' {
		if (file.deletions === 0 && file.additions > 0 && file.oldPath === 'unknown') return 'added';
		if (file.additions === 0 && file.deletions > 0) return 'deleted';
		return 'modified';
	}

	const isLargeDiff = $derived(
		parsedDiff && (parsedDiff.files.length > 100 || parsedDiff.totalLines > 10000)
	);

	const fileCount = $derived(parsedDiff?.files.length ?? 0);

	// Auto-expand and fetch when autoExpand is true (used on the PR page)
	$effect(() => {
		if (autoExpand && hasPR && !expanded && !fetched && !loading) {
			expanded = true;
			fetchDiff();
		}
	});
</script>

{#if hasPR}
	<div class="rounded-xl border shadow-sm overflow-hidden">
		<!-- Toggle Button -->
		<button
			type="button"
			class="w-full flex items-center gap-2 px-5 py-3 text-left hover:bg-muted/50 transition-colors"
			onclick={toggleExpanded}
		>
			<FileText class="w-4 h-4 text-muted-foreground shrink-0" />
			<span class="font-semibold text-sm">
				{#if fetched && parsedDiff}
					View Changes ({fileCount} {fileCount === 1 ? 'file' : 'files'})
				{:else}
					View Changes
				{/if}
			</span>
			<ChevronDown
				class="w-4 h-4 text-muted-foreground ml-auto transition-transform {expanded
					? 'rotate-180'
					: ''}"
			/>
		</button>

		{#if expanded}
			<div class="border-t">
				<!-- Loading State -->
				{#if loading}
					<div class="flex items-center justify-center gap-2 py-8">
						<Loader2 class="w-4 h-4 animate-spin text-muted-foreground" />
						<span class="text-sm text-muted-foreground">Loading diff...</span>
					</div>

				<!-- Error State -->
				{:else if error}
					<div class="flex items-center gap-2 px-5 py-4 text-sm text-red-600 dark:text-red-400">
						<AlertTriangle class="w-4 h-4 shrink-0" />
						<span>{error}</span>
					</div>

				<!-- Empty Diff -->
				{:else if parsedDiff && parsedDiff.files.length === 0}
					<div class="px-5 py-4 text-sm text-muted-foreground">No changes</div>

				<!-- Large Diff Warning -->
				{:else if isLargeDiff}
					<div class="px-5 py-4 space-y-3">
						<div
							class="flex items-center gap-2 text-sm text-amber-600 dark:text-amber-400"
						>
							<AlertTriangle class="w-4 h-4 shrink-0" />
							<span>
								This diff is very large ({fileCount} files, {parsedDiff?.totalLines.toLocaleString()} lines).
								It may be easier to view on GitHub.
							</span>
						</div>
						{#if prUrl}
							<a
								href="{prUrl}/files"
								class="inline-flex items-center gap-1.5 text-sm text-primary hover:underline"
								target="_blank"
								rel="noopener noreferrer"
							>
								View on GitHub
								<ExternalLink class="w-3.5 h-3.5" />
							</a>
						{/if}
						<!-- Still render the diff below the warning -->
						<div class="border-t pt-3">
							{@render diffContent()}
						</div>
					</div>

				<!-- Normal Diff -->
				{:else if parsedDiff}
					{@render diffContent()}
				{/if}
			</div>
		{/if}
	</div>
{/if}

{#snippet diffContent()}
	<div class="flex">
		<!-- File Navigator Sidebar (hidden on mobile) -->
		{#if showFileNav && parsedDiff && parsedDiff.files.length > 0}
			<div class="hidden sm:block w-64 shrink-0 border-r border-border bg-muted/20 overflow-y-auto max-h-[calc(100vh-200px)] sticky top-0">
				<div class="flex items-center justify-between px-3 py-2 border-b border-border bg-muted/30">
					<span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Files</span>
					<span class="text-xs text-muted-foreground">{parsedDiff.files.length}</span>
				</div>
				<nav class="py-1">
					{#each parsedDiff.files as file, fileIndex}
						<button
							type="button"
							class="w-full text-left px-3 py-1.5 text-xs hover:bg-muted/50 transition-colors flex items-center gap-2 group"
							onclick={() => scrollToFile(fileIndex)}
						>
							{#if getFileStatus(file) === 'added'}
								<FilePlus class="w-3.5 h-3.5 text-green-600 dark:text-green-400 shrink-0" />
							{:else if getFileStatus(file) === 'deleted'}
								<FileX class="w-3.5 h-3.5 text-red-600 dark:text-red-400 shrink-0" />
							{:else}
								<FileEdit class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
							{/if}
							<span class="truncate font-mono">
								<span class="text-muted-foreground/60">{getFileDir(file.newPath)}</span><span class="text-foreground">{getFileName(file.newPath)}</span>
							</span>
							<span class="ml-auto shrink-0 flex items-center gap-1 text-[10px] opacity-0 group-hover:opacity-100 transition-opacity">
								{#if file.additions > 0}
									<span class="text-green-600 dark:text-green-400">+{file.additions}</span>
								{/if}
								{#if file.deletions > 0}
									<span class="text-red-600 dark:text-red-400">-{file.deletions}</span>
								{/if}
							</span>
						</button>
					{/each}
				</nav>
			</div>
		{/if}

		<!-- Diff Content -->
		<div class="flex-1 min-w-0 divide-y divide-border">
			<!-- Sidebar toggle -->
			{#if parsedDiff && parsedDiff.files.length > 0}
				<div class="flex items-center px-3 py-1.5 bg-muted/20 border-b border-border">
					<button
						type="button"
						class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
						onclick={() => (showFileNav = !showFileNav)}
					>
						<List class="w-3.5 h-3.5" />
						{showFileNav ? 'Hide' : 'Show'} file tree
					</button>
				</div>
			{/if}

			{#each parsedDiff!.files as file, fileIndex}
				<div id="diff-file-{fileIndex}">
					<!-- File Header -->
					<button
						type="button"
						class="w-full flex items-center gap-2 px-4 py-2 text-left bg-muted/30 hover:bg-muted/50 transition-colors text-xs font-mono"
						onclick={() => toggleFile(fileIndex)}
					>
						<ChevronRight
							class="w-3.5 h-3.5 text-muted-foreground shrink-0 transition-transform {file.expanded
								? 'rotate-90'
								: ''}"
						/>
						<span class="text-muted-foreground truncate">
							{#if file.oldPath === file.newPath}
								{file.newPath}
							{:else}
								{file.oldPath} &rarr; {file.newPath}
							{/if}
						</span>
						<span class="ml-auto shrink-0 flex items-center gap-2 text-[11px]">
							{#if file.additions > 0}
								<span class="text-green-600 dark:text-green-400">+{file.additions}</span>
							{/if}
							{#if file.deletions > 0}
								<span class="text-red-600 dark:text-red-400">-{file.deletions}</span>
							{/if}
						</span>
					</button>

					<!-- File Content -->
					{#if file.expanded}
						<div class="overflow-x-auto">
							<table class="w-full text-xs font-mono border-collapse">
								<tbody>
									{#each file.hunks as hunk}
										{#each hunk.lines as line}
											{#if line.type === 'hunk-header'}
												<tr class="bg-muted/40">
													<td
														class="px-2 py-0.5 text-right text-muted-foreground select-none w-[1%] whitespace-nowrap border-r border-border"
													></td>
													<td
														class="px-3 py-1 text-muted-foreground italic"
													>
														{line.content}
													</td>
												</tr>
											{:else if line.type === 'addition'}
												<tr class="bg-green-500/15 dark:bg-green-500/10">
													<td
														class="px-2 py-0 text-right text-muted-foreground/60 select-none w-[1%] whitespace-nowrap border-r border-green-500/30"
													>
														{line.newLineNum}
													</td>
													<td
														class="px-3 py-0 whitespace-pre border-l-2 border-green-500"
													><span class="text-green-700 dark:text-green-400 select-none">+</span>{line.content}</td>
												</tr>
											{:else if line.type === 'deletion'}
												<tr class="bg-red-500/15 dark:bg-red-500/10">
													<td
														class="px-2 py-0 text-right text-muted-foreground/60 select-none w-[1%] whitespace-nowrap border-r border-red-500/30"
													>
														{line.oldLineNum}
													</td>
													<td
														class="px-3 py-0 whitespace-pre border-l-2 border-red-500"
													><span class="text-red-700 dark:text-red-400 select-none">-</span>{line.content}</td>
												</tr>
											{:else}
												<tr class="hover:bg-muted/30">
													<td
														class="px-2 py-0 text-right text-muted-foreground/60 select-none w-[1%] whitespace-nowrap border-r border-border"
													>
														{line.newLineNum}
													</td>
													<td
														class="px-3 py-0 whitespace-pre border-l-2 border-transparent"
													><span class="select-none">&nbsp;</span>{line.content}</td>
												</tr>
											{/if}
										{/each}
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	</div>
{/snippet}

<script lang="ts">
	import { renderMarkdown } from '$lib/markdown';

	let {
		content,
		class: className = ''
	}: { content: string; class?: string } = $props();
</script>

<div class="prose prose-sm dark:prose-invert max-w-none {className}">
	{@html renderMarkdown(content)}
</div>

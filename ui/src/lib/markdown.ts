import { marked, Renderer } from 'marked';
import DOMPurify from 'dompurify';

const renderer = new Renderer();

// Make links open in new tabs
const originalLinkRenderer = renderer.link.bind(renderer);
renderer.link = function (token) {
	const html = originalLinkRenderer(token);
	return html.replace('<a ', '<a target="_blank" rel="noopener noreferrer" ');
};

marked.setOptions({
	breaks: true,
	gfm: true,
	renderer
});

export function renderMarkdown(content: string): string {
	const html = marked(content) as string;
	return DOMPurify.sanitize(html, {
		ADD_ATTR: ['target']
	});
}

export function stripMarkdown(content: string): string {
	return content
		.replace(/#{1,6}\s+/g, '')
		.replace(/\*\*(.+?)\*\*/g, '$1')
		.replace(/\*(.+?)\*/g, '$1')
		.replace(/__(.+?)__/g, '$1')
		.replace(/_(.+?)_/g, '$1')
		.replace(/~~(.+?)~~/g, '$1')
		.replace(/`{1,3}[^`]*`{1,3}/g, (match) => match.replace(/`/g, ''))
		.replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
		.replace(/!\[([^\]]*)\]\([^)]+\)/g, '$1')
		.replace(/^\s*[-*+]\s+/gm, '')
		.replace(/^\s*\d+\.\s+/gm, '')
		.replace(/^\s*>\s+/gm, '')
		.replace(/---+/g, '')
		.replace(/\n{2,}/g, ' ')
		.trim();
}

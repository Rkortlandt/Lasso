<script lang="ts">
	import DOMPurify from "dompurify";
	import { cleanCanvasHtml } from "$lib/canvasCleaner";

	interface Props {
		html?: string;
	}

	let { html = "" }: Props = $props();

	const domPurifyConfig = {
		ALLOWED_TAGS: [
			"p",
			"h1",
			"h2",
			"h3",
			"h4",
			"h5",
			"h6",
			"ul",
			"ol",
			"li",
			"strong",
			"em",
			"b",
			"i",
			"a",
			"table",
			"thead",
			"tbody",
			"tr",
			"td",
			"th",
			"br",
			"code",
			"pre",
			"blockquote",
			"hr",
			"span",
		],
		FORBID_ATTR: ["style", "class", "width", "height"],
		ADD_ATTR: ["target", "rel"],
	};

	// Remove screenreader-only / external icon elements before sanitizing
	DOMPurify.addHook("uponSanitizeElement", (node) => {
		if (node.nodeType === Node.ELEMENT_NODE) {
			const el = node as Element;
			const className = el.getAttribute("class") || "";
			const title = el.getAttribute("title") || "";
			if (
				className.includes("screenreader-only") ||
				className.includes("ui-icon-extlink") ||
				className.includes("ui-icon") ||
				/Links to (?:an\s+)?external site/i.test(title)
			) {
				el.remove();
				return;
			}
		}
	});

	// Ensure links open in a new tab safely and clean text nodes inside anchor tags
	DOMPurify.addHook("afterSanitizeAttributes", (node) => {
		if (node.tagName === "A") {
			node.setAttribute("target", "_blank");
			node.setAttribute("rel", "noopener noreferrer");
		}
	});

	const sanitizedHtml = $derived.by(() => {
		if (!html) return "";
		const preCleaned = cleanCanvasHtml(html);
		const sanitized = DOMPurify.sanitize(preCleaned, domPurifyConfig);
		return cleanCanvasHtml(sanitized);
	});
</script>

<div class="canvas-content">
	{@html sanitizedHtml}
</div>

<style>
	.canvas-content {
		font-size: 0.75rem; /* text-xs */
		line-height: 1.5;
		color: var(--foreground);
		word-break: break-word;
	}

	/* Slightly smaller, well-proportioned titles matching UI aesthetics */
	.canvas-content :global(h1) {
		font-size: 1rem; /* 14px */
		font-weight: 600;
		color: var(--foreground);
		margin-top: 0.75rem;
		margin-bottom: 0.375rem;
		line-height: 1.35;
	}

	.canvas-content :global(h2) {
		font-size: 1.2rem; /* 13px */
		font-weight: 600;
		color: var(--foreground);
		margin-top: 0.25rem;
		margin-bottom: 0.25rem;
		line-height: 1.35;
	}

	.canvas-content :global(h3),
	.canvas-content :global(h4),
	.canvas-content :global(h5),
	.canvas-content :global(h6) {
		font-size: 0.9rem; /* 12px */
		font-weight: 600;
		color: var(--foreground);
		margin-top: 0.5rem;
		margin-bottom: 0.25rem;
		line-height: 1.35;
	}

	.canvas-content :global(p) {
		margin-bottom: 0.5rem;
		color: var(--foreground);
		opacity: 0.9;
	}

	.canvas-content :global(p:last-child) {
		margin-bottom: 0;
	}

	.canvas-content :global(a) {
		color: var(--primary);
		text-decoration: underline;
		text-underline-offset: 2px;
		font-weight: 500;
		word-break: break-all;
		transition: opacity 0.15s ease;
	}

	.canvas-content :global(a:hover) {
		opacity: 0.8;
	}

	.canvas-content :global(ul) {
		list-style-type: disc;
		padding-left: 1.25rem;
		margin-bottom: 0.5rem;
	}

	.canvas-content :global(ol) {
		list-style-type: decimal;
		padding-left: 1.25rem;
		margin-bottom: 0.5rem;
	}

	.canvas-content :global(li) {
		margin-bottom: 0.25rem;
	}

	.canvas-content :global(table) {
		width: 100%;
		border-collapse: collapse;
		margin-top: 0.5rem;
		margin-bottom: 0.5rem;
		font-size: 0.75rem;
		border: 1px solid var(--border);
		border-radius: 0.375rem;
		overflow: hidden;
	}

	.canvas-content :global(th) {
		background-color: color-mix(in srgb, var(--muted) 90%, transparent);
		color: var(--muted-foreground);
		font-weight: 600;
		text-align: left;
		padding: 0.375rem 0.625rem;
		border: 1px solid var(--border);
		font-size: 0.6875rem;
	}

	.canvas-content :global(td) {
		border: 1px solid var(--border);
		padding: 0.375rem 0.625rem;
		color: var(--foreground);
		background-color: color-mix(in srgb, var(--muted) 70%, transparent);
	}

	.canvas-content :global(blockquote) {
		border-left: 2px solid var(--primary);
		padding-left: 0.75rem;
		margin: 0.5rem 0;
		font-style: italic;
		color: var(--muted-foreground);
	}

	.canvas-content :global(code) {
		font-family: var(--mono);
		font-size: 0.6875rem;
		background-color: color-mix(in srgb, var(--muted) 60%, transparent);
		border: 1px solid var(--border);
		padding: 0.125rem 0.25rem;
		border-radius: 0.25rem;
	}

	.canvas-content :global(pre) {
		font-family: var(--mono);
		font-size: 0.6875rem;
		background-color: color-mix(in srgb, var(--muted) 50%, transparent);
		border: 1px solid var(--border);
		padding: 0.5rem;
		border-radius: 0.375rem;
		overflow-x: auto;
		margin: 0.5rem 0;
	}

	.canvas-content :global(strong),
	.canvas-content :global(b) {
		font-weight: 600;
		color: var(--foreground);
	}

	.canvas-content :global(hr) {
		border: 0;
		border-top: 1px solid var(--border);
		margin: 0.75rem 0;
	}
</style>

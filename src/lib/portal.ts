import type { Action } from "svelte/action";

export const portal: Action<HTMLElement, HTMLElement | string | undefined> = (
	node,
	target = "body",
) => {
	let targetEl: HTMLElement | null = null;

	function update(newTarget: HTMLElement | string | undefined = "body") {
		if (typeof newTarget === "string") {
			targetEl = document.querySelector(newTarget);
		} else if (newTarget instanceof HTMLElement) {
			targetEl = newTarget;
		} else {
			targetEl = document.body;
		}

		if (targetEl && node.parentNode !== targetEl) {
			targetEl.appendChild(node);
		}
	}

	function destroy() {
		if (node.parentNode) {
			node.parentNode.removeChild(node);
		}
	}

	update(target);

	return {
		update,
		destroy,
	};
};

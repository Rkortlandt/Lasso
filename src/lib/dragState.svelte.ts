export interface DragTaskPayload {
	taskId: string;
	taskName: string;
	calendarId: string;
	color: string;
	courseName: string;
}

class DragState {
	activeTask = $state<DragTaskPayload | null>(null);
	isDragging = $state(false);

	setTask(task: DragTaskPayload | null) {
		this.activeTask = task;
		this.isDragging = Boolean(task);
	}

	clear() {
		this.activeTask = null;
		this.isDragging = false;
	}
}

export const dragState = new DragState();

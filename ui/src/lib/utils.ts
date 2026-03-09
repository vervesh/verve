import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export type WithElementRef<T, E extends Element = HTMLElement> = T & {
	ref?: E | null;
};

export type WithoutChild<T> = Omit<T, 'child'>;

export type WithoutChildrenOrChild<T> = Omit<T, 'children' | 'child'>;

export function taskUrl(owner: string, name: string, number: number): string {
	return `/${owner}/${name}/tasks/${number}`;
}

export function epicUrl(owner: string, name: string, number: number): string {
	return `/${owner}/${name}/epics/${number}`;
}

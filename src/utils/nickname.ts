export function getNickname(user: {
	first_name: string;
	last_name?: string;
}): string {
	return user.last_name
		? `${user.first_name} ${user.last_name}`
		: user.first_name;
}

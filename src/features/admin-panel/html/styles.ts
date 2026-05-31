export function renderStyles(): string {
	return `
@keyframes spin { to { transform: rotate(360deg); } }
.spinner {
	width: 18px; height: 18px;
	border: 2px solid rgba(255,255,255,0.1);
	border-top-color: white;
	border-radius: 50%;
	animation: spin 0.6s linear infinite;
}
@keyframes fadeInUp {
	from { opacity: 0; transform: translateY(12px); }
	to { opacity: 1; transform: translateY(0); }
}
@keyframes slideInLeft {
	from { transform: translateX(-100%); }
	to { transform: translateX(0); }
}
@keyframes slideInDown {
	from { opacity: 0; transform: translateY(-20px); }
	to { opacity: 1; transform: translateY(0); }
}
@keyframes fadeOut {
	from { opacity: 1; }
	to { opacity: 0; }
}
@keyframes fadeScale {
	from { opacity: 0; }
	to { opacity: 1; }
}
.login-enter { animation: fadeInUp 0.4s ease; }
.sidebar-enter { animation: slideInLeft 0.25s ease; }
.toast-enter { animation: slideInDown 0.3s ease; }
.toast-exit { animation: fadeOut 0.3s ease forwards; }
.island { animation: fadeScale 0.25s ease both; }
.island:nth-child(2) { animation-delay: 0.04s; }
.island:nth-child(3) { animation-delay: 0.08s; }
body[data-state="app"] #login-page { display: none !important; }
body[data-state="login"] #sidebar { display: none !important; }
body[data-state="login"] #sidebar-backdrop { display: none !important; }
body[data-state="login"] #main-wrapper { display: none !important; }
::-webkit-scrollbar { width: 5px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.1); border-radius: 3px; }
::-webkit-scrollbar-thumb:hover { background: rgba(255,255,255,0.2); }
`;
}

// Closes an open card menu on an outside click or Escape.
function closeMenus(except) {
  document.querySelectorAll("details[data-menu][open]").forEach(function (menu) {
    if (menu !== except) menu.removeAttribute("open");
  });
}
document.addEventListener("click", function (event) {
  closeMenus(event.target.closest("details[data-menu]"));
});
document.addEventListener("keydown", function (event) {
  if (event.key === "Escape") closeMenus(null);
});

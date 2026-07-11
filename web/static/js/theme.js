// waldi theme toggle: light -> dark -> back. Runs before paint (loaded in <head>).
(function () {
  var saved = localStorage.getItem("waldi-theme");
  if (saved) document.documentElement.dataset.theme = saved;

  window.toggleTheme = function () {
    var root = document.documentElement;
    var dark = root.dataset.theme === "dark" ||
      (!root.dataset.theme && matchMedia("(prefers-color-scheme: dark)").matches);
    var next = dark ? "light" : "dark";
    root.dataset.theme = next;
    localStorage.setItem("waldi-theme", next);
  };
})();

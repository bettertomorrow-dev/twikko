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

// Date, time and date-time pickers. The browser draws a native picker outside
// the page, where no CSS reaches, so a button opens a popover built here. The
// visible input is a masked text field, so a value can also be typed. The
// picker's hidden input carries the form value, in the format the native input
// used. Handlers are delegated, because htmx replaces a whole card on Reset and
// the pickers inside it come and go.
(function () {
  var MONTHS = ["January", "February", "March", "April", "May", "June", "July",
    "August", "September", "October", "November", "December"];
  var WEEKDAYS = ["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"];
  var current = null;

  var NAV_CLASS = "flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground";
  var TEXT_BUTTON_CLASS = "rounded-md px-2 py-1 text-xs font-medium text-muted-foreground hover:bg-accent hover:text-accent-foreground";
  var DAY_CLASS = "size-8 rounded-md text-sm hover:bg-accent hover:text-accent-foreground";
  var DAY_OUTSIDE_CLASS = " text-muted-foreground/50";
  var DAY_TODAY_CLASS = " bg-accent text-accent-foreground font-medium";
  var DAY_SELECTED_CLASS = " bg-primary text-primary-foreground font-medium hover:bg-primary hover:text-primary-foreground";
  var CELL_CLASS = "h-8 w-12 shrink-0 rounded-md text-sm hover:bg-accent hover:text-accent-foreground";

  // Input masks. A segment is its digit count, and the highest first digit that
  // still leaves a valid value: a higher one gets a zero in front of it, so
  // typing 9 into the hour gives 09.
  var YEAR = { n: 4 }, MONTH = { n: 2, max: 1 }, DAY = { n: 2, max: 3 };
  var HOUR = { n: 2, max: 2 }, MINUTE = { n: 2, max: 5 };
  var MASKS = {
    date: [YEAR, "-", MONTH, "-", DAY],
    time: [HOUR, ":", MINUTE],
    datetime: [YEAR, "-", MONTH, "-", DAY, " ", HOUR, ":", MINUTE]
  };
  var FULL = {
    date: /^(\d{4})-(\d{2})-(\d{2})$/,
    time: /^(\d{2}):(\d{2})$/,
    datetime: /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/
  };

  function pad(n) { return String(n).padStart(2, "0"); }

  function dateKey(d) { return d.y + "-" + pad(d.m + 1) + "-" + pad(d.d); }

  function today() {
    var now = new Date();
    return { y: now.getFullYear(), m: now.getMonth(), d: now.getDate() };
  }

  // Reads a form value: 2026-10-20, 14:30 or 2026-10-20T14:30, with optional seconds.
  function parse(kind, value) {
    var date = null, time = null, match;
    if (kind !== "time") {
      match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
      if (match) date = { y: +match[1], m: +match[2] - 1, d: +match[3] };
    }
    if (kind !== "date") {
      match = /(?:^|T)(\d{2}):(\d{2})(:\d{2})?$/.exec(value);
      if (match) time = { h: +match[1], min: +match[2], sec: match[3] || "" };
    }
    return { date: date, time: time };
  }

  // The form value of a date and time. It is empty when neither is set.
  function compose(kind, date, time) {
    var clock = time ? pad(time.h) + ":" + pad(time.min) + time.sec : "";
    if (kind === "date") return date ? dateKey(date) : "";
    if (kind === "time") return clock;
    return date || time ? dateKey(date || today()) + "T" + (clock || "00:00") : "";
  }

  // The text in the input for a form value. It matches pickerText in data.go.
  function textFor(kind, value) {
    var parsed = parse(kind, value);
    var parts = [];
    if (kind !== "time" && parsed.date) parts.push(dateKey(parsed.date));
    if (kind !== "date" && parsed.time) parts.push(pad(parsed.time.h) + ":" + pad(parsed.time.min));
    return parts.length ? parts.join(" ") : value;
  }

  // Lays the digits of what was typed into the mask.
  function maskText(kind, raw) {
    var digits = raw.replace(/\D/g, "");
    var out = "", used = 0;
    MASKS[kind].forEach(function (part) {
      if (used >= digits.length) return;
      if (typeof part === "string") {
        out += part;
        return;
      }
      var chunk = "";
      while (chunk.length < part.n && used < digits.length) {
        if (chunk.length === 0 && part.max !== undefined && +digits[used] > part.max) chunk = "0";
        chunk += digits[used++];
      }
      out += chunk;
    });
    return out;
  }

  // Reads a full mask back into a date and time, or null when it is not a real one.
  function readText(kind, text) {
    var match = FULL[kind].exec(text);
    if (!match) return null;
    var nums = match.slice(1).map(Number);
    var date = null, time = null;
    if (kind !== "time") {
      var check = new Date(nums[0], nums[1] - 1, nums[2]);
      if (check.getFullYear() !== nums[0] || check.getMonth() !== nums[1] - 1 || check.getDate() !== nums[2]) return null;
      date = { y: nums[0], m: nums[1] - 1, d: nums[2] };
    }
    if (kind !== "date") {
      var h = nums[nums.length - 2], min = nums[nums.length - 1];
      if (h > 23 || min > 59) return null;
      time = { h: h, min: min, sec: "" };
    }
    return { date: date, time: time };
  }

  function fieldOf(element) {
    var root = element.closest("[data-picker]");
    return {
      root: root, kind: root.dataset.picker,
      hidden: root.querySelector('input[type="hidden"]'),
      input: root.querySelector("[data-picker-input]"),
      trigger: root.querySelector("[data-picker-trigger]"),
      icons: root.dataset.icons, required: "required" in root.dataset
    };
  }

  // Stores a form value and tells htmx. The visible text follows it unless the
  // user is typing it.
  function setValue(f, value, typing) {
    if (!typing) {
      var text = textFor(f.kind, value);
      if (f.input.value !== text) f.input.value = text;
      f.input.removeAttribute("aria-invalid");
    }
    if (f.hidden.value !== value) {
      f.hidden.value = value;
      f.hidden.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }

  function icon(s, name) {
    return '<span class="icon" style="--icon:url(' + s.icons + name + '.svg)" aria-hidden="true"></span>';
  }

  function calendarMarkup(s) {
    var first = new Date(s.view.y, s.view.m, 1);
    var offset = (first.getDay() + 6) % 7;
    var now = today();
    var focusKey = dateKey(s.focus);
    var html = '<div class="w-[17.5rem] shrink-0">' +
      '<div class="mb-2 flex items-center justify-between">' +
      '<button type="button" class="' + NAV_CLASS + '" data-act="nav" data-delta="-1" data-fk="prev" aria-label="Previous month">' + icon(s, "chevron-left") + "</button>" +
      '<div class="text-sm font-medium" aria-live="polite">' + MONTHS[s.view.m] + " " + s.view.y + "</div>" +
      '<button type="button" class="' + NAV_CLASS + '" data-act="nav" data-delta="1" data-fk="next" aria-label="Next month">' + icon(s, "chevron-right") + "</button>" +
      "</div>" +
      '<div class="grid grid-cols-7 place-items-center">';
    WEEKDAYS.forEach(function (name) {
      html += '<div class="flex size-8 items-center justify-center text-xs text-muted-foreground">' + name + "</div>";
    });
    for (var i = 0; i < 42; i++) {
      var cell = new Date(s.view.y, s.view.m, 1 - offset + i);
      var day = { y: cell.getFullYear(), m: cell.getMonth(), d: cell.getDate() };
      var key = dateKey(day);
      var cls = DAY_CLASS;
      if (day.m !== s.view.m) cls += DAY_OUTSIDE_CLASS;
      if (key === dateKey(now)) cls += DAY_TODAY_CLASS;
      if (s.date && key === dateKey(s.date)) cls += DAY_SELECTED_CLASS;
      html += '<button type="button" class="' + cls + '" data-act="day" data-key="' + key + '" data-fk="d:' + key + '"' +
        ' tabindex="' + (key === focusKey ? 0 : -1) + '"' +
        (s.date && key === dateKey(s.date) ? ' aria-pressed="true"' : "") + ">" + day.d + "</button>";
    }
    return html + "</div></div>";
  }

  function columnMarkup(kind, count, selected, title) {
    var html = '<div class="relative h-64 w-14 shrink-0 overflow-y-auto [scrollbar-width:thin]" data-col="' + kind + '" role="listbox" aria-label="' + title + '">';
    for (var i = 0; i < count; i++) {
      var cls = CELL_CLASS + (i === selected ? DAY_SELECTED_CLASS : "");
      html += '<button type="button" class="' + cls + '" data-act="' + kind + '" data-value="' + i + '" data-fk="' + kind + ":" + i + '"' +
        ' role="option" aria-selected="' + (i === selected) + '"' + (i === selected ? ' data-selected="true"' : "") + ">" + pad(i) + "</button>";
    }
    return html + "</div>";
  }

  function render() {
    var s = current, pop = s.pop;
    var scroll = {};
    pop.querySelectorAll("[data-col]").forEach(function (col) { scroll[col.dataset.col] = col.scrollTop; });
    var active = pop.contains(document.activeElement) ? document.activeElement.dataset.fk : null;

    var html = '<div class="flex gap-3">';
    if (s.kind !== "time") html += calendarMarkup(s);
    if (s.kind !== "date") {
      html += '<div class="flex gap-1' + (s.kind === "datetime" ? " border-l pl-3" : "") + '">' +
        columnMarkup("hour", 24, s.time ? s.time.h : -1, "Hour") +
        columnMarkup("min", 60, s.time ? s.time.min : -1, "Minute") + "</div>";
    }
    html += "</div>" +
      '<div class="mt-2 flex items-center justify-between border-t pt-2">' +
      '<button type="button" class="' + TEXT_BUTTON_CLASS + '" data-act="now" data-fk="now">' + (s.kind === "date" ? "Today" : "Now") + "</button>" +
      (s.required ? "" : '<button type="button" class="' + TEXT_BUTTON_CLASS + '" data-act="clear" data-fk="clear">Clear</button>') +
      "</div>";
    pop.innerHTML = html;

    // Keep each column where the user left it, unless its selected value is out
    // of view: Now, Clear-then-pick or typing moves the selection, and it has to
    // stay visible.
    pop.querySelectorAll("[data-col]").forEach(function (col) {
      col.scrollTop = scroll[col.dataset.col] || 0;
      var chosen = col.querySelector("[data-selected]");
      if (!chosen) return;
      var top = chosen.offsetTop, bottom = top + chosen.offsetHeight;
      if (top < col.scrollTop || bottom > col.scrollTop + col.clientHeight) {
        col.scrollTop = top - (col.clientHeight - chosen.offsetHeight) / 2;
      }
    });
    if (active) {
      var again = pop.querySelector('[data-fk="' + active + '"]');
      if (again) again.focus();
    }
  }

  function position() {
    var s = current;
    var box = s.root.getBoundingClientRect();
    var pop = s.pop;
    // Leave room for the fixed save bar when deciding whether to open upward.
    var room = window.innerHeight - 72 - box.bottom;
    var top = box.bottom + 4;
    if (pop.offsetHeight > room && box.top > pop.offsetHeight + 8) top = box.top - pop.offsetHeight - 4;
    var left = Math.max(8, Math.min(box.left, window.innerWidth - pop.offsetWidth - 8));
    pop.style.top = top + "px";
    pop.style.left = left + "px";
  }

  function commit() {
    setValue(current, compose(current.kind, current.date, current.time), false);
  }

  function close(focusInput) {
    if (!current) return;
    var s = current;
    current = null;
    s.pop.remove();
    s.trigger.setAttribute("aria-expanded", "false");
    if (focusInput) s.input.focus();
  }

  // With focusPopup the keyboard moves into the popover. A click in the input
  // opens it without, so the user can keep typing.
  function open(f, focusPopup) {
    var parsed = parse(f.kind, f.hidden.value);
    var anchor = parsed.date || today();
    var pop = document.createElement("div");
    pop.className = "fixed z-50 rounded-md border bg-popover p-3 text-popover-foreground shadow-md outline-none";
    pop.setAttribute("role", "dialog");
    pop.setAttribute("aria-label", f.input.getAttribute("aria-label") || "Pick a value");
    current = Object.assign({}, f, {
      pop: pop, date: parsed.date, time: parsed.time,
      view: { y: anchor.y, m: anchor.m }, focus: anchor
    });
    document.body.appendChild(pop);
    f.trigger.setAttribute("aria-expanded", "true");
    render();
    position();
    if (focusPopup) focusStart();
  }

  function focusStart() {
    var start = current.pop.querySelector('[tabindex="0"]') || current.pop.querySelector("[data-selected]") || current.pop.querySelector("button");
    if (start) start.focus();
  }

  function setView(y, m) {
    var d = new Date(y, m, 1);
    current.view = { y: d.getFullYear(), m: d.getMonth() };
  }

  function moveFocus(delta) {
    var s = current;
    var next = new Date(s.focus.y, s.focus.m, s.focus.d + delta);
    s.focus = { y: next.getFullYear(), m: next.getMonth(), d: next.getDate() };
    setView(s.focus.y, s.focus.m);
    render();
    var button = s.pop.querySelector('[data-fk="d:' + dateKey(s.focus) + '"]');
    if (button) button.focus();
  }

  function act(button) {
    var s = current;
    switch (button.dataset.act) {
      case "nav":
        setView(s.view.y, s.view.m + +button.dataset.delta);
        s.focus = { y: s.view.y, m: s.view.m, d: 1 };
        render();
        return;
      case "day":
        var picked = parse("date", button.dataset.key).date;
        s.date = picked;
        s.focus = picked;
        setView(picked.y, picked.m);
        if (s.kind === "datetime" && !s.time) s.time = { h: 0, min: 0, sec: "" };
        break;
      case "hour":
        s.time = { h: +button.dataset.value, min: s.time ? s.time.min : 0, sec: s.time ? s.time.sec : "" };
        if (s.kind === "datetime" && !s.date) s.date = today();
        break;
      case "min":
        s.time = { h: s.time ? s.time.h : 0, min: +button.dataset.value, sec: s.time ? s.time.sec : "" };
        if (s.kind === "datetime" && !s.date) s.date = today();
        break;
      case "now":
        var now = new Date();
        s.date = today();
        s.time = s.kind === "date" ? null : { h: now.getHours(), min: now.getMinutes(), sec: "" };
        s.focus = s.date;
        setView(s.date.y, s.date.m);
        break;
      case "clear":
        s.date = null;
        s.time = null;
        commit();
        close(true);
        return;
    }
    commit();
    if (s.kind === "date") {
      close(true);
    } else {
      render();
    }
  }

  // Typing: lay the digits into the mask, and store the value once the mask is
  // full and real. Emptying the field stores an empty value.
  function typed(input) {
    var f = fieldOf(input);
    var before = input.value.slice(0, input.selectionStart).replace(/\D/g, "").length;
    var atEnd = input.selectionStart === input.value.length;
    var text = maskText(f.kind, input.value);
    if (text !== input.value) {
      input.value = text;
      var caret = text.length;
      if (!atEnd) {
        for (var i = 0, seen = 0; i < text.length && seen < before; i++) {
          if (/\d/.test(text[i])) seen++;
          caret = i + 1;
        }
      }
      input.setSelectionRange(caret, caret);
    }

    var live = current && current.root === f.root ? current : null;
    var date = null, time = null;
    if (text !== "") {
      var read = readText(f.kind, text);
      var full = FULL[f.kind].test(text);
      if (full && !read) input.setAttribute("aria-invalid", "true"); else input.removeAttribute("aria-invalid");
      if (!read) { setValue(f, text, true); return; }
      date = read.date;
      time = read.time;
      // Typing does not touch seconds that the minute still matches.
      var old = parse(f.kind, f.hidden.value).time;
      if (time && old && old.h === time.h && old.min === time.min) time.sec = old.sec;
    } else {
      input.removeAttribute("aria-invalid");
    }
    setValue(f, compose(f.kind, date, time), true);
    if (live) {
      live.date = date;
      live.time = time;
      if (date) {
        live.focus = date;
        setView(date.y, date.m);
      }
      render();
    }
  }

  document.addEventListener("input", function (event) {
    if (event.target.matches && event.target.matches("[data-picker-input]")) typed(event.target);
  });

  // Tabbing out of the picker closes its popover.
  document.addEventListener("focusin", function (event) {
    if (current && !current.pop.contains(event.target) && !current.root.contains(event.target)) close(false);
  });

  document.addEventListener("click", function (event) {
    var trigger = event.target.closest("[data-picker-trigger]");
    if (trigger) {
      var f = fieldOf(trigger);
      var wasOpen = current && current.root === f.root;
      close(false);
      if (!wasOpen) open(f, true);
      return;
    }
    var input = event.target.closest("[data-picker-input]");
    if (input) {
      var typedInto = fieldOf(input);
      if (!(current && current.root === typedInto.root)) {
        close(false);
        open(typedInto, false);
      }
      return;
    }
    if (!current) return;
    var button = event.target.closest("[data-act]");
    if (button && current.pop.contains(button)) {
      act(button);
    } else if (!current.pop.contains(event.target)) {
      close(false);
    }
  });

  document.addEventListener("keydown", function (event) {
    if (event.target.matches && event.target.matches("[data-picker-input]")) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        if (!current || current.root !== event.target.closest("[data-picker]")) {
          close(false);
          open(fieldOf(event.target), true);
        } else {
          focusStart();
        }
        return;
      }
      if (event.key === "Enter" && current) {
        event.preventDefault();
        close(false);
        return;
      }
    }
    if (!current) return;
    if (event.key === "Escape") {
      close(true);
      return;
    }
    var onDay = event.target.dataset && event.target.dataset.act === "day";
    var step = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }[event.key];
    if (onDay && step) {
      event.preventDefault();
      moveFocus(step);
    } else if (onDay && (event.key === "PageUp" || event.key === "PageDown")) {
      event.preventDefault();
      var from = new Date(current.focus.y, current.focus.m, current.focus.d);
      var shifted = new Date(current.focus.y, current.focus.m + (event.key === "PageUp" ? -1 : 1), current.focus.d);
      moveFocus(Math.round((shifted - from) / 864e5));
    }
  });

  window.addEventListener("resize", function () { if (current) position(); });
  window.addEventListener("scroll", function () { if (current) position(); }, true);
  // Reset replaces a card, and the picker with it.
  document.addEventListener("htmx:afterSettle", function () {
    if (current && !document.body.contains(current.root)) close(false);
  });
})();

// Update state without replacing the editor or its focus.
document.addEventListener("htmx:afterSettle", function (event) {
  var root = event.detail.target;
  var statuses = root.matches("[data-field-status]") ? [root] : root.querySelectorAll("[data-field-status]");
  statuses.forEach(function (status) {
    var wrapper = status.closest("[data-field-control]");
    if (!wrapper) return;
    wrapper.classList.toggle("border-changed", status.dataset.changed === "true");
    wrapper.classList.toggle("border-transparent", status.dataset.changed !== "true");
    wrapper.querySelectorAll("[aria-describedby]").forEach(function (input) {
      input.setAttribute("aria-invalid", status.dataset.invalid);
    });
  });
});

function transitionRoot(element, target) {
  var path = element.getAttribute("hx-post") || "";
  if (!/\/(save$|cancel$|reset$|collapsed$)/.test(path)) return null;
  var selector = element.getAttribute("hx-target");
  var root = target || (selector && document.querySelector(selector));
  return !root || root === document.body ? document.getElementById("entities") : root;
}

function lockTransition(root) {
  if (!root) return;
  root.inert = true;
  root.setAttribute("aria-busy", "true");
}

// Lock as soon as an action is queued: later input must not be detached by it.
document.addEventListener("htmx:confirm", function (event) {
  lockTransition(transitionRoot(event.detail.elt));
});
document.addEventListener("htmx:beforeRequest", function (event) {
  var root = transitionRoot(event.detail.elt, event.detail.target);
  lockTransition(root);
  event.detail.xhr.twikkoLockedRoot = root;
});
document.addEventListener("htmx:afterRequest", function (event) {
  var root = event.detail.xhr.twikkoLockedRoot;
  if (!root) return;
  root.inert = false;
  root.removeAttribute("aria-busy");
});

// Table operations move existing nodes, keeping drafts and expanded editors intact.
(function () {
  var resizeDrag = null, columnDrag = null, keyboardDrag = null;
  var hovered = null, crosshairCells = [], shiftHeld = false;
  var MIN_WIDTH = 96;
  var highlightClasses = ["row-highlight-neutral", "row-highlight-info", "row-highlight-success", "row-highlight-warning", "row-highlight-danger"];

  function table() { return document.getElementById("data-table"); }
  function announce(message) {
    var status = document.querySelector("[data-table-announcement]");
    if (status) status.textContent = message;
  }
  function keyed(container, key) {
    return Array.from(container.children).find(function (element) { return element.dataset.columnKey === key; });
  }
  function order(grid) {
    return Array.from(grid.querySelector("colgroup").children).map(function (col) { return col.dataset.columnKey; });
  }
  function restoreFocus(active) {
    if (active && active.isConnected && document.activeElement !== active) active.focus({ preventScroll: true });
  }
  function applyOrder(grid, keys) {
    var active = document.activeElement;
    [grid.querySelector("colgroup")].concat(Array.from(grid.rows)).forEach(function (row) {
      var cells = new Map(Array.from(row.children).map(function (cell) { return [cell.dataset.columnKey, cell]; }));
      keys.forEach(function (key) { if (cells.has(key)) row.appendChild(cells.get(key)); });
    });
    restoreFocus(active);
    clearHover();
  }
  function resize(handle, width) {
    var grid = handle.closest("table");
    var column = keyed(grid.querySelector("colgroup"), handle.dataset.columnResize);
    if (!column) return;
    width = Math.max(MIN_WIDTH, Math.round(width));
    column.style.width = width + "px";
    grid.style.width = Array.from(grid.querySelector("colgroup").children).reduce(function (total, col) {
      return total + parseFloat(col.style.width);
    }, 0) + "px";
    handle.setAttribute("aria-valuenow", String(width));
  }
  function cellText(cell) {
    var summary = cell.querySelector("details > summary");
    if (summary) return [summary.textContent];
    var inputs = Array.from(cell.querySelectorAll("input:not([type=hidden]),select,textarea"));
    if (!inputs.length) return [cell.textContent];
    return inputs.flatMap(function (input) {
      if (input.tagName === "SELECT") return Array.from(input.options).map(function (option) { return option.textContent; });
      if (input.type === "checkbox") return [input.closest("label") ? input.closest("label").textContent : ""];
      return [input.value || input.placeholder || ""];
    });
  }
  function autoFit(handle) {
    var grid = handle.closest("table"), key = handle.dataset.columnResize;
    var canvas = document.createElement("canvas"), context = canvas.getContext("2d");
    if (!context) return;
    var width = MIN_WIDTH;
    Array.from(grid.rows).forEach(function (row) {
      var cell = keyed(row, key);
      if (!cell) return;
      var style = getComputedStyle(cell.querySelector("input,select,textarea,summary") || cell);
      context.font = style.font;
      cellText(cell).forEach(function (text) {
        String(text).split(/\r?\n/).forEach(function (line) {
          var padding = cell.tagName === "TH" || cell.querySelector("[data-picker-input]") ? 72 : 32;
          width = Math.max(width, context.measureText(line.trim()).width + padding);
        });
      });
    });
    resize(handle, Math.min(width, Math.max(200, Math.min(800, window.innerWidth * 0.75))));
    announce("Column fitted to content");
  }

  function clearHover() {
    if (hovered) hovered.removeAttribute("data-cell-hover");
    hovered = null;
    crosshairCells.forEach(function (cell) { cell.removeAttribute("data-crosshair"); });
    crosshairCells = [];
  }
  function showHover(cell) {
    if (hovered === cell && (!cell || (crosshairCells.length > 0) === shiftHeld)) return;
    clearHover();
    if (!cell || resizeDrag || columnDrag || keyboardDrag) return;
    hovered = cell;
    cell.dataset.cellHover = "true";
    if (!shiftHeld) return;
    var grid = cell.closest("table"), row = cell.parentElement, key = cell.dataset.columnKey;
    Array.from(grid.tBodies[0].rows).forEach(function (item) {
      var columnCell = keyed(item, key);
      if (columnCell) crosshairCells.push(columnCell);
    });
    Array.from(row.cells).forEach(function (item) { if (item !== cell) crosshairCells.push(item); });
    crosshairCells.forEach(function (item) { item.dataset.crosshair = "true"; });
  }
  function updateShift(held) {
    if (shiftHeld === held) return;
    shiftHeld = held;
    var cell = hovered;
    clearHover();
    showHover(cell);
  }

  function clearDrop(grid) {
    grid.querySelectorAll("[data-column-drop]").forEach(function (header) { header.removeAttribute("data-column-drop"); });
  }
  function moveColumn(grid, source, target, after) {
    var keys = order(grid).filter(function (key) { return key !== source; });
    var targetIndex = keys.indexOf(target);
    if (targetIndex < 1 || target === "actions") return;
    keys.splice(targetIndex + (after ? 1 : 0), 0, source);
    applyOrder(grid, keys);
  }
  function finishKeyboard(cancel) {
    if (!keyboardDrag) return;
    var state = keyboardDrag;
    keyboardDrag = null;
    state.handle.removeAttribute("aria-pressed");
    state.grid.removeAttribute("data-column-moving");
    if (cancel) applyOrder(state.grid, state.original);
    restoreFocus(state.handle);
    announce(cancel ? "Column move cancelled" : "Column moved");
  }

  document.addEventListener("pointerdown", function (event) {
    if (event.button !== 0) return;
    var handle = event.target.closest("[data-column-resize]");
    if (handle) {
      event.preventDefault();
      finishKeyboard(false);
      resizeDrag = { handle: handle, x: event.clientX, width: Number(handle.getAttribute("aria-valuenow")), pointer: event.pointerId };
      handle.closest("table").dataset.columnMoving = "true";
      clearHover();
      handle.setPointerCapture(event.pointerId);
      return;
    }
    handle = event.target.closest("[data-column-drag]");
    if (!handle) return;
    event.preventDefault();
    finishKeyboard(false);
    handle.focus({ preventScroll: true });
    columnDrag = { handle: handle, grid: handle.closest("table"), key: handle.dataset.columnDrag,
      x: event.clientX, pointer: event.pointerId, moved: false, target: null, after: false };
    handle.setPointerCapture(event.pointerId);
    clearHover();
  });
  document.addEventListener("pointermove", function (event) {
    if (resizeDrag && resizeDrag.pointer === event.pointerId) {
      resize(resizeDrag.handle, resizeDrag.width + event.clientX - resizeDrag.x);
      return;
    }
    if (columnDrag && columnDrag.pointer === event.pointerId) {
      var state = columnDrag;
      if (!state.moved && Math.abs(event.clientX - state.x) < 4) return;
      state.moved = true;
      state.grid.dataset.columnMoving = "true";
      clearDrop(state.grid);
      var element = document.elementFromPoint(event.clientX, event.clientY);
      var header = element && element.closest("th[data-column-key]");
      state.target = null;
      if (!header || header.closest("table") !== state.grid) return;
      var key = header.dataset.columnKey;
      if (key === "number" || key === "actions" || key === state.key) return;
      var box = header.getBoundingClientRect();
      state.target = key;
      state.after = event.clientX >= box.x + box.width / 2;
      header.dataset.columnDrop = state.after ? "after" : "before";
      var scroll = state.grid.parentElement, bounds = scroll.getBoundingClientRect();
      if (event.clientX > bounds.right - 40) scroll.scrollLeft += 24;
      if (event.clientX < bounds.left + 40) scroll.scrollLeft -= 24;
      return;
    }
    updateShift(event.shiftKey);
    var cell = event.target.closest("#data-table tbody td[data-column-key]");
    showHover(cell);
  });
  function endPointer(event) {
    if (resizeDrag && resizeDrag.pointer === event.pointerId) {
      var resizeState = resizeDrag;
      resizeDrag = null;
      resizeState.handle.closest("table").removeAttribute("data-column-moving");
      if (resizeState.handle.hasPointerCapture(event.pointerId)) resizeState.handle.releasePointerCapture(event.pointerId);
    }
    if (columnDrag && columnDrag.pointer === event.pointerId) {
      var state = columnDrag;
      columnDrag = null;
      clearDrop(state.grid);
      state.grid.removeAttribute("data-column-moving");
      if (event.type === "pointerup" && state.moved && state.target) {
        moveColumn(state.grid, state.key, state.target, state.after);
        restoreFocus(state.handle);
        announce("Column moved");
      }
      if (state.handle.hasPointerCapture(event.pointerId)) state.handle.releasePointerCapture(event.pointerId);
    }
  }
  document.addEventListener("pointerup", endPointer);
  document.addEventListener("pointercancel", endPointer);
  document.addEventListener("lostpointercapture", endPointer);
  document.addEventListener("pointerout", function (event) {
    if (hovered && (!event.relatedTarget || !hovered.closest("table").contains(event.relatedTarget))) clearHover();
  });
  window.addEventListener("blur", function () {
    shiftHeld = false;
    clearHover();
    finishKeyboard(true);
    if (resizeDrag) endPointer({ pointerId: resizeDrag.pointer, type: "pointercancel" });
    if (columnDrag) endPointer({ pointerId: columnDrag.pointer, type: "pointercancel" });
  });
  document.addEventListener("dblclick", function (event) {
    var handle = event.target.closest("[data-column-resize]");
    if (!handle) return;
    event.preventDefault();
    autoFit(handle);
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Shift") updateShift(true);
    var handle = event.target.closest("[data-column-resize]");
    if (handle && ["ArrowLeft", "ArrowRight", "Home", "Enter"].includes(event.key)) {
      event.preventDefault();
      if (event.key === "Enter") { autoFit(handle); return; }
      var width = Number(handle.getAttribute("aria-valuenow")), step = event.shiftKey ? 64 : 16;
      resize(handle, event.key === "Home" ? MIN_WIDTH : width + (event.key === "ArrowRight" ? step : -step));
      return;
    }
    handle = event.target.closest("[data-column-drag]");
    if (!handle) return;
    if (!keyboardDrag && (event.key === " " || event.key === "Enter")) {
      event.preventDefault();
      var grid = handle.closest("table");
      keyboardDrag = { handle: handle, grid: grid, original: order(grid), key: handle.dataset.columnDrag };
      handle.setAttribute("aria-pressed", "true");
      grid.dataset.columnMoving = "true";
      clearHover();
      announce("Column selected. Use left and right arrows, Enter to finish, Escape to cancel.");
    } else if (keyboardDrag && ["ArrowLeft", "ArrowRight", " ", "Enter", "Escape"].includes(event.key)) {
      event.preventDefault();
      if ([" ", "Enter", "Escape"].includes(event.key)) { finishKeyboard(event.key === "Escape"); return; }
      var keys = order(keyboardDrag.grid), position = keys.indexOf(keyboardDrag.key);
      var next = position + (event.key === "ArrowLeft" ? -1 : 1);
      if (next > 0 && next < keys.length - 1) {
        keys.splice(position, 1);
        keys.splice(next, 0, keyboardDrag.key);
        applyOrder(keyboardDrag.grid, keys);
        announce("Column position " + next);
      }
    }
  });
  document.addEventListener("keyup", function (event) { if (event.key === "Shift") updateShift(false); });
  document.addEventListener("focusout", function (event) {
    if (keyboardDrag && event.target === keyboardDrag.handle) {
      setTimeout(function () {
        if (keyboardDrag && document.activeElement !== keyboardDrag.handle) finishKeyboard(false);
      }, 0);
    }
  });

  document.addEventListener("click", function (event) {
    var button = event.target.closest("[data-column-sort]");
    if (!button) return;
    event.preventDefault();
    var grid = button.closest("table"), header = button.closest("th");
    if (grid.dataset.sortPending === "true") return;
    grid.dataset.sortPending = "true";
    var direction = header.getAttribute("aria-sort") === "ascending" ? "desc" : "asc";
    var url = grid.dataset.sortUrl + "?column=" + encodeURIComponent(button.dataset.columnSort) + "&direction=" + direction;
    grid.setAttribute("aria-busy", "true");
    htmx.ajax("POST", url, {
      source: button, target: grid, swap: "none",
      handler: function (_, response) {
        if (response.xhr.status < 200 || response.xhr.status >= 300) throw new Error("Unable to sort table");
        var result = JSON.parse(response.xhr.responseText), body = grid.tBodies[0];
        var rows = new Map(Array.from(body.rows).map(function (row) { return [Number(row.dataset.entityIndex), row]; }));
        if (!Array.isArray(result.order) || result.order.length !== rows.size || new Set(result.order).size !== rows.size || result.order.some(function (index) { return !rows.has(index); })) throw new Error("Invalid table order");
        var active = document.activeElement;
        result.order.forEach(function (index) { body.appendChild(rows.get(index)); });
        restoreFocus(active);
        grid.querySelectorAll("thead th[aria-sort]").forEach(function (item) { item.setAttribute("aria-sort", "none"); });
        header.setAttribute("aria-sort", direction === "asc" ? "ascending" : "descending");
        clearHover();
        announce(button.getAttribute("aria-label") + ", " + (direction === "asc" ? "ascending" : "descending"));
      }
    }).catch(function () { announce("Unable to sort table. Try again."); }).finally(function () {
      grid.removeAttribute("aria-busy");
      delete grid.dataset.sortPending;
    });
  });

  function applyHighlight(index, className) {
    var row = document.getElementById("entity-" + index);
    if (!row || row.tagName !== "TR") return;
    row.classList.remove.apply(row.classList, highlightClasses);
    if (highlightClasses.includes(className)) row.classList.add(className);
  }
  document.addEventListener("tableHighlight", function (event) {
    applyHighlight(event.detail.index, event.detail.className);
  });
  document.addEventListener("htmx:afterSwap", function (event) {
    var grid = table();
    if (!grid) return;
    var row = event.target.closest("tr[data-entity-index]");
    if (row && row.isConnected) {
      var keys = order(grid);
      var cells = new Map(Array.from(row.cells).map(function (cell) { return [cell.dataset.columnKey, cell]; }));
      if (keys.some(function (key, i) { return row.cells[i] !== cells.get(key); })) {
        keys.forEach(function (key) { if (cells.has(key)) row.appendChild(cells.get(key)); });
      }
    }
    if (hovered && !hovered.isConnected) clearHover();
  });
  function refreshHighlights() {
    var grid = table();
    if (!grid || grid.dataset.relativeHighlights !== "true" || document.hidden) return;
    htmx.ajax("GET", grid.dataset.highlightsUrl, {
      source: grid, target: grid, swap: "none",
      handler: function (_, response) {
        if (response.xhr.status !== 200) return;
        var result = JSON.parse(response.xhr.responseText);
        Object.keys(result.highlights).forEach(function (index) { applyHighlight(index, result.highlights[index]); });
      }
    }).catch(function () { /* Keep the last known colors until the next refresh. */ });
  }
  setTimeout(function tick() {
    refreshHighlights();
    setTimeout(tick, 60000 - Date.now() % 60000);
  }, 60000 - Date.now() % 60000);
  document.addEventListener("visibilitychange", function () { if (!document.hidden) refreshHighlights(); });
})();

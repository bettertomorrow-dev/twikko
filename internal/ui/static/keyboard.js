// Keyboard navigation stays on card headers and table cells until editing begins.
(function () {
  var isolated = null;
  var pendingCard = null;
  var activeCellElement = null;
  var restoreCellFocus = false;
  var cellDraft = null;
  var controls = 'a[href], button, input:not([type="hidden"]), select, textarea, summary, [tabindex]';
  var postedControl = '[hx-post][name="value"]';

  function available(root) {
    return Array.from(root.querySelectorAll(controls)).filter(function (el) {
      return !el.disabled && el.getClientRects().length && !el.closest('[inert]') &&
        !el.matches('[data-card-nav], [data-table-cell], [tabindex="-1"]');
    });
  }
  function isolate(card) {
    isolated = card.id;
    card.setAttribute('data-isolated', '');
    var items = available(card);
    var field = items.find(function (el) { return el.matches('input:not([type="hidden"]), select, textarea'); });
    (field || items[0] || card.querySelector('[data-card-nav]')).focus();
  }
  function leaveCard() {
    var card = isolated && document.getElementById(isolated);
    isolated = null;
    if (card) {
      card.removeAttribute('data-isolated');
      card.querySelector('[data-card-nav]').focus();
    }
  }
  function cardToggle(card, enter) {
    var nav = card.querySelector('[data-card-nav]');
    pendingCard = { id: card.id, enter: enter, nav: nav };
    card.querySelector('[data-card-toggle]').click();
  }
  function enumStep(card, direction) {
    var select = card.querySelector('select[data-arrow-field]');
    if (!select) return;
    var options = Array.from(select.options).filter(function (option) { return !option.disabled && option.value !== ''; });
    var index = options.findIndex(function (option) { return option.value === select.value; });
    var next = index < 0 ? (direction > 0 ? 0 : options.length - 1) : index + direction;
    if (next < 0 || next >= options.length) return;
    select.value = options[next].value;
    select.setAttribute('data-arrow-changing', '');
    select.dispatchEvent(new Event('change', { bubbles: true }));
    select.removeAttribute('data-arrow-changing');
  }
  function cells() { return Array.from(document.querySelectorAll('#data-table tbody [data-table-cell]')); }
  function activate(cell, focus) {
    if (!cell) return;
    if (activeCellElement && activeCellElement !== cell) activeCellElement.tabIndex = -1;
    cell.tabIndex = 0;
    activeCellElement = cell;
    if (focus) {
      cell.focus();
      var table = cell.closest('table');
      var header = table.tHead.querySelector('th').getBoundingClientRect();
      var bounds = cell.getBoundingClientRect();
      if (bounds.top < header.bottom) window.scrollBy(0, bounds.top - header.bottom);
    }
  }
  function restoreCells() {
    var all = cells();
    if (!all.length) return;
    var cell = activeCellElement && document.getElementById(activeCellElement.id);
    activate(cell || all[0], false);
    all.forEach(function (item) {
      if (item.hasAttribute('data-keyboard-ready')) return;
      item.setAttribute('data-keyboard-ready', '');
      disableEditorTabbing(item);
    });
  }
  function disableEditorTabbing(item) {
    item.querySelectorAll(controls).forEach(function (el) {
        if (!el.hasAttribute('data-cell-tabindex')) {
          el.dataset.cellTabindex = el.hasAttribute('tabindex') ? el.getAttribute('tabindex') : '';
        }
        el.tabIndex = -1;
    });
  }
  function allowEditorTabbing(cell) {
    cell.querySelectorAll('[data-cell-tabindex]').forEach(function (el) {
      var original = el.dataset.cellTabindex;
      if (original === '') el.removeAttribute('tabindex');
      else el.setAttribute('tabindex', original);
    });
  }
  function allowEditor(cell) {
    var editor = cell.querySelector('input:not([type="hidden"]):not(:disabled), select:not(:disabled), textarea:not(:disabled)');
    if (!editor) return;
    var details = cell.querySelector('details');
    if (details) details.open = true;
    allowEditorTabbing(cell);
    editor.focus();
    if (editor.matches('input[type="text"]:not([inputmode]), textarea')) editor.select();
  }
  function editorCell(target) {
    var popup = target && target.closest('[data-picker-anchor]');
    var control = popup ? document.getElementById(popup.dataset.pickerAnchor) : target;
    return control && control.closest('[data-field-control]') && control.closest('[data-table-cell]');
  }
  function editState(control) {
    if (control.type === 'checkbox') return control.checked;
    if (control.tagName === 'SELECT') return control.selectedIndex;
    return control.value;
  }
  function beginEdit(cell) {
    cellDraft = { cell: cell, controls: Array.from(cell.querySelectorAll(postedControl + ', [data-picker-input]')).map(function (control) {
      return { control: control, value: editState(control), invalid: control.getAttribute('aria-invalid') };
    }) };
  }
  function finishEdit(apply, focusCell) {
    var draft = cellDraft;
    if (!draft || draft.finishing) return;
    draft.finishing = true;
    if (!apply) {
      draft.controls.forEach(function (entry) {
        var control = entry.control;
        if (control.type === 'checkbox') control.checked = entry.value;
        else if (control.tagName === 'SELECT') control.selectedIndex = entry.value;
        else control.value = entry.value;
        if (control.matches('[data-picker-input]')) {
          if (entry.invalid === null) control.removeAttribute('aria-invalid');
          else control.setAttribute('aria-invalid', entry.invalid);
        }
      });
    }
    // Keep native blur/change events buffered until focus has left the editor.
    if (focusCell) {
      activate(draft.cell, true);
      disableEditorTabbing(draft.cell);
    }
    cellDraft = null;
    if (!apply || !draft.cell.isConnected) return;
    var changed = draft.controls.find(function (entry) {
      return entry.control.matches(postedControl) && editState(entry.control) !== entry.value;
    });
    if (!changed) return;
    var source = changed.control;
    var values = [source.value];
    if (source.matches('[role="switch"]')) values = [String(source.checked)];
    else if (source.type === 'checkbox') {
      values = draft.controls.filter(function (entry) { return entry.control.checked; })
        .map(function (entry) { return entry.control.value; });
    }
    // Freeze the applied value now. htmx may queue the request while another edit runs.
    source.dispatchEvent(new CustomEvent('change', { bubbles: true, detail: { cellValues: values } }));
  }

  document.addEventListener('htmx:confirm', function (event) {
    var trigger = event.detail.triggeringEvent;
    if (cellDraft && editorCell(event.detail.elt) === cellDraft.cell &&
        !(trigger && trigger.detail && trigger.detail.cellValues)) event.preventDefault();
  });
  document.addEventListener('htmx:configRequest', function (event) {
    var trigger = event.detail.triggeringEvent;
    var values = trigger && trigger.detail && trigger.detail.cellValues;
    if (values) {
      if (values.length) event.detail.parameters.value = values;
      else delete event.detail.parameters.value;
    }
  });
  // Handle Escape before the picker receives it. Picker buttons still use
  // Enter to choose a value, and Shift+Enter can insert a textarea newline.
  document.addEventListener('keydown', function (event) {
    if (!cellDraft || editorCell(event.target) !== cellDraft.cell || event.isComposing || event.altKey) return;
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z') {
      var cell = cellDraft.cell;
      finishEdit(true, false);
      beginEdit(cell);
      return;
    }
    if (event.ctrlKey || event.metaKey) return;
    if (event.key === 'Escape' || (event.key === 'Enter' && !event.shiftKey && !event.target.matches('button'))) {
      event.preventDefault();
      event.stopImmediatePropagation();
      finishEdit(event.key === 'Enter', true);
    }
  }, true);

  function moveCell(cell, key) {
    var row = cell.parentElement;
    var rowCells = Array.from(row.querySelectorAll('[data-table-cell]'));
    var rows = Array.from(row.parentElement.rows);
    var column = rowCells.indexOf(cell), rowIndex = rows.indexOf(row);
    if (key === 'ArrowLeft') column--;
    if (key === 'ArrowRight') column++;
    if (key === 'ArrowUp') rowIndex--;
    if (key === 'ArrowDown') rowIndex++;
    if (rowIndex < 0 || rowIndex >= rows.length || column < 0 || column >= rowCells.length) return;
    activate(rows[rowIndex].querySelectorAll('[data-table-cell]')[column], true);
  }
  document.addEventListener('keydown', function (event) {
    if (event.defaultPrevented || event.isComposing || event.altKey || event.ctrlKey || event.metaKey) return;
    var target = event.target;
    var vertical = !event.shiftKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown');
    if (vertical && (target === document.body || target === document.documentElement)) {
      var first = document.querySelector('[data-card-nav]');
      if (first && !first.closest('[inert]')) {
        event.preventDefault();
        first.focus();
      }
      return;
    }
    var nav = target.matches('[data-card-nav]') ? target : null;
    if (nav) {
      var card = nav.closest('[data-card]');
      if (vertical) {
        event.preventDefault();
        var cards = Array.from(document.querySelectorAll('[data-card]'));
        var next = cards.indexOf(card) + (event.key === 'ArrowUp' ? -1 : 1);
        if (cards[next]) cards[next].querySelector('[data-card-nav]').focus();
      } else if (event.shiftKey && event.key === 'Backspace') {
        event.preventDefault();
        if (!event.repeat) cardToggle(card, false);
      } else if (!event.shiftKey && event.key === 'Enter') {
        event.preventDefault();
        if (card.querySelector('[data-card-toggle]').getAttribute('aria-expanded') === 'false') cardToggle(card, true);
        else isolate(card);
      } else if (!event.shiftKey && (event.key === 'ArrowLeft' || event.key === 'ArrowRight')) {
        if (!event.repeat) enumStep(card, event.key === 'ArrowRight' ? 1 : -1);
        event.preventDefault();
      }
      return;
    }
    if (isolated) {
      var current = document.getElementById(isolated);
      var popover = target.closest('[data-picker-anchor]');
      var anchor = popover && document.getElementById(popover.dataset.pickerAnchor);
      if (current && anchor && current.contains(anchor) && event.key === 'Tab') {
        event.preventDefault();
        var cardItems = available(current);
        var trigger = anchor.closest('[data-picker]').querySelector('[data-picker-trigger]');
        var at = cardItems.indexOf(event.shiftKey ? anchor : trigger);
        var destination = (at + (event.shiftKey ? -1 : 1) + cardItems.length) % cardItems.length;
        cardItems[destination].focus();
        return;
      }
      if (current && current.contains(target)) {
        if (event.key === 'Escape') { event.preventDefault(); leaveCard(); return; }
        if (event.key === 'Tab') {
          var items = available(current);
          if (!items.length) { event.preventDefault(); return; }
          var index = items.indexOf(target);
          if (event.shiftKey && index <= 0) { event.preventDefault(); items[items.length - 1].focus(); }
          else if (!event.shiftKey && index === items.length - 1) { event.preventDefault(); items[0].focus(); }
        }
        return;
      }
    }
    var cell = target.closest('[data-table-cell]');
    if (!cell) return;
    if (target === cell && !event.shiftKey && ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) {
      event.preventDefault(); moveCell(cell, event.key);
    } else if (target === cell && event.key === 'Enter') {
      event.preventDefault(); allowEditor(cell);
    } else if (target !== cell && event.key === 'Escape') {
      event.preventDefault(); activate(cell, true); disableEditorTabbing(cell);
    }
  });
  document.addEventListener('pointerdown', function (event) {
    var picker = event.target.closest('[data-picker-anchor]');
    var anchor = picker && document.getElementById(picker.dataset.pickerAnchor);
    var cardOwner = anchor && anchor.closest('[data-card]');
    if (isolated && !event.target.closest('#' + CSS.escape(isolated)) && (!cardOwner || cardOwner.id !== isolated)) {
      var card = document.getElementById(isolated);
      if (card) card.removeAttribute('data-isolated');
      isolated = null;
    }
    var cell = event.target.closest('[data-table-cell]');
    if (cell) { activate(cell, false); allowEditorTabbing(cell); }
  });
  document.addEventListener('focusin', function (event) {
    var card = event.target.closest('[data-card]');
    if (card && card.id === isolated) card.setAttribute('data-isolated', '');
    var cell = event.target.closest('[data-table-cell]');
    if (cell) activate(cell, false);
    var editor = editorCell(event.target);
    if (cellDraft && editor !== cellDraft.cell) finishEdit(true, false);
    if (!cellDraft && editor && editor.querySelector(postedControl)) beginEdit(editor);
  });
  document.addEventListener('focusout', function (event) {
    var cell = event.target.closest('[data-table-cell]');
    if (cellDraft && editorCell(event.target) === cellDraft.cell && editorCell(event.relatedTarget) !== cellDraft.cell &&
        (event.relatedTarget || !event.target.closest('[data-picker-anchor]'))) finishEdit(true, false);
    if (cell) setTimeout(function () {
      if (!cell.contains(document.activeElement)) disableEditorTabbing(cell);
    }, 0);
  });
  document.addEventListener('htmx:beforeCleanupElement', function (event) {
    var element = event.detail.elt;
    if (cellDraft && element.contains(cellDraft.cell)) cellDraft = null;
    if (activeCellElement && element.contains(activeCellElement) && document.activeElement === activeCellElement) {
      restoreCellFocus = true;
    }
  });
  document.addEventListener('htmx:afterSettle', function () {
    if (isolated) {
      var isolatedCard = document.getElementById(isolated);
      if (isolatedCard && isolatedCard.contains(document.activeElement)) isolatedCard.setAttribute('data-isolated', '');
      else if (!isolatedCard || document.activeElement !== document.body) isolated = null;
    }
    if (pendingCard) {
      var pending = pendingCard;
      var card = document.getElementById(pending.id);
      if (card && !pending.nav.isConnected) {
        pendingCard = null;
        var nav = card.querySelector('[data-card-nav]');
        if (document.activeElement === nav || document.activeElement === document.body) {
          if (pending.enter) isolate(card);
          else nav.focus();
        }
      }
    }
    restoreCells();
    if (restoreCellFocus && document.activeElement === document.body && activeCellElement && activeCellElement.isConnected) {
      activate(activeCellElement, true);
    }
    restoreCellFocus = false;
  });
  restoreCells();
  if (activeCellElement && document.activeElement === document.body) {
    activate(activeCellElement, true);
  }
})();

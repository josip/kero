const SELECTED_TIMEFRAME = (new URLSearchParams(window.location.search)).get('t') || 't';

function updateTimeframeSelector() {
    const options = document.querySelectorAll('#timeframe-selector a');
    for (let option of options) {
        const href = option.getAttribute('href');
        if (href === `?t=${SELECTED_TIMEFRAME}`) {
            option.classList.add('active');
            document.getElementById('selected-timeframe-label').innerHTML = option.innerHTML;
            return;
        }
    }
}

function showAllTableItemsInModal(e) {
    e.preventDefault();

    const table = e.target.closest('table');
    const title = e.target.closest('article').querySelector('h6').innerText;
    const modal = document.createElement('dialog');
    modal.open = true;
    modal.innerHTML = `
    <article>
        <header>
            <a href="#" class="close" aria-label="Close"></a>
            <strong>${title}</strong>
        </header>

        ${table.outerHTML}
    </article>`;
    document.body.appendChild(modal);
}

function closeModal(e) {
    const dialog = e.target.closest('dialog');
    if (dialog) {
        e.preventDefault();
        document.body.removeChild(dialog);
    }
}

function initShowAllInModal() {
    document.body.addEventListener('click', (e) => {
        const target = e.target;
        if (target.nodeName === 'A' && target.classList.contains('show-all')) {
            showAllTableItemsInModal(e);
        }
        if (target.nodeName === 'A' && target.classList.contains('close')) {
            closeModal(e);
        }
    });
}

function localizeNumbers() {
    const nums = document.querySelectorAll('[data-localize-number]');
    for (const el of nums) {
        el.innerHTML = parseInt(el.innerHTML, 10).toLocaleString()
    }
}

window.addEventListener('DOMContentLoaded', () => {
    initShowAllInModal();
    updateTimeframeSelector();
    localizeNumbers();
}, false);

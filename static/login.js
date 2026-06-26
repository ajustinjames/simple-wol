(function () {
    'use strict';

    async function handleSetup(e) {
        e.preventDefault();
        var errEl = document.getElementById('setup-error');
        errEl.hidden = true;

        var password = document.getElementById('setup-password').value;
        var confirm = document.getElementById('setup-confirm').value;
        if (password !== confirm) {
            errEl.textContent = 'Passwords do not match.';
            errEl.hidden = false;
            return;
        }

        try {
            var res = await fetch('/api/setup', {
                method: 'POST',
                headers: {'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest'},
                body: JSON.stringify({
                    username: document.getElementById('setup-username').value,
                    password: password,
                }),
            });
            if (!res.ok) {
                var data = await res.json();
                throw new Error(data.error || 'Setup failed');
            }
            window.location.href = '/login?setup=success';
        } catch (err) {
            errEl.textContent = err.message;
            errEl.hidden = false;
        }
    }

    async function handleLogin(e) {
        e.preventDefault();
        var errEl = document.getElementById('login-error');
        errEl.hidden = true;

        try {
            var res = await fetch('/api/login', {
                method: 'POST',
                headers: {'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest'},
                body: JSON.stringify({
                    username: document.getElementById('login-username').value,
                    password: document.getElementById('login-password').value,
                    remember: document.getElementById('login-remember').checked,
                }),
            });
            if (!res.ok) {
                var data = await res.json();
                throw new Error(data.error || 'Login failed');
            }
            window.location.href = '/';
        } catch (err) {
            errEl.textContent = err.message;
            errEl.hidden = false;
        }
    }

    // Show setup success message if redirected after account creation
    if (new URLSearchParams(window.location.search).get('setup') === 'success') {
        var el = document.getElementById('setup-success');
        if (el) el.hidden = false;
    }

    // Wire up form handlers via addEventListener (CSP-safe, no inline handlers)
    // Only one form is visible depending on setup state, so guard for null.
    document.addEventListener('DOMContentLoaded', function () {
        var setupForm = document.getElementById('setup-form');
        if (setupForm) {
            setupForm.addEventListener('submit', handleSetup);
        }

        var loginForm = document.getElementById('login-form');
        if (loginForm) {
            loginForm.addEventListener('submit', handleLogin);
        }
    });
})();

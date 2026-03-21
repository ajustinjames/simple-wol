(function () {
    'use strict';

    let devices = [];

    // --- Helpers ---

    function escapeHtml(str) {
        var div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    async function api(url, options) {
        options = options || {};
        options.headers = options.headers || {};
        options.headers['X-Requested-With'] = 'XMLHttpRequest';
        var res = await fetch(url, options);
        if (res.status === 401) {
            window.location.href = '/login';
            return null;
        }
        return res;
    }

    // --- Device CRUD ---

    async function loadDevices() {
        var res = await api('/api/devices');
        if (!res) return;
        devices = await res.json();
        if (!Array.isArray(devices)) devices = [];
        renderDevices();
    }

    function renderDevices() {
        var container = document.getElementById('device-list');

        if (devices.length === 0) {
            container.textContent = '';
            var msg = document.createElement('p');
            msg.style.color = '#888';
            msg.style.textAlign = 'center';
            msg.style.padding = '2rem';
            msg.textContent = 'No devices yet. Add one to get started.';
            container.appendChild(msg);
            return;
        }

        // Build table using DOM methods to avoid innerHTML with user data
        container.textContent = '';

        var table = document.createElement('table');
        table.className = 'device-table';

        var thead = document.createElement('thead');
        var headerRow = document.createElement('tr');
        ['Status', 'Name', 'MAC', 'IP', 'Actions'].forEach(function (text) {
            var th = document.createElement('th');
            th.textContent = text;
            headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);
        table.appendChild(thead);

        var tbody = document.createElement('tbody');

        devices.forEach(function (d) {
            var tr = document.createElement('tr');
            tr.id = 'device-row-' + d.id;

            // Status cell
            var statusTd = document.createElement('td');
            var statusSpan = document.createElement('span');
            statusSpan.id = 'status-' + d.id;
            statusSpan.className = 'status-indicator';
            statusSpan.title = 'unknown';
            statusTd.appendChild(statusSpan);
            tr.appendChild(statusTd);

            // Name cell
            var nameTd = document.createElement('td');
            nameTd.textContent = d.name;
            tr.appendChild(nameTd);

            // MAC cell
            var macTd = document.createElement('td');
            macTd.textContent = d.mac_address;
            tr.appendChild(macTd);

            // IP cell
            var ipTd = document.createElement('td');
            ipTd.textContent = d.ip_address;
            tr.appendChild(ipTd);

            // Actions cell
            var actionsTd = document.createElement('td');

            var wakeBtn = document.createElement('button');
            wakeBtn.className = 'btn btn-primary btn-sm';
            wakeBtn.textContent = 'Wake';
            wakeBtn.addEventListener('click', function () { wakeDevice(d.id); });
            actionsTd.appendChild(wakeBtn);

            actionsTd.appendChild(document.createTextNode(' '));

            var editBtn = document.createElement('button');
            editBtn.className = 'btn btn-secondary btn-sm';
            editBtn.textContent = 'Edit';
            editBtn.addEventListener('click', function () { editDevice(d.id); });
            actionsTd.appendChild(editBtn);

            actionsTd.appendChild(document.createTextNode(' '));

            var deleteBtn = document.createElement('button');
            deleteBtn.className = 'btn btn-danger btn-sm';
            deleteBtn.textContent = 'Delete';
            deleteBtn.addEventListener('click', function () { deleteDevice(d.id); });
            actionsTd.appendChild(deleteBtn);

            tr.appendChild(actionsTd);
            tbody.appendChild(tr);
        });

        table.appendChild(tbody);
        container.appendChild(table);

        // Check status for all devices
        devices.forEach(function (d) {
            checkStatus(d.id).then(function (status) {
                updateStatusIndicator(d.id, status);
            });
        });
    }

    function updateStatusIndicator(id, status) {
        var el = document.getElementById('status-' + id);
        if (!el) return;
        el.className = 'status-indicator';
        if (status === 'online') {
            el.classList.add('online');
            el.title = 'Online';
        } else if (status === 'waking') {
            el.classList.add('waking');
            el.title = 'Waking...';
        } else {
            el.title = 'Offline';
        }
    }

    function toggleAddForm() {
        var form = document.getElementById('add-form');
        form.hidden = !form.hidden;
    }

    async function addDevice() {
        var name = document.getElementById('add-name').value.trim();
        var mac = document.getElementById('add-mac').value.trim();
        var ip = document.getElementById('add-ip').value.trim();
        var port = parseInt(document.getElementById('add-port').value, 10) || 9;
        if (!name || !mac) {
            alert('Name and MAC address are required.');
            return;
        }

        var res = await api('/api/devices', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                name: name,
                mac_address: mac,
                ip_address: ip,
                port: port,
            }),
        });

        if (!res) return;
        if (!res.ok) {
            var data = await res.json();
            alert(data.error || 'Failed to add device');
            return;
        }

        // Reset form and hide
        document.getElementById('add-name').value = '';
        document.getElementById('add-mac').value = '';
        document.getElementById('add-ip').value = '';
        document.getElementById('add-port').value = '9';
        document.getElementById('add-form').hidden = true;

        await loadDevices();
    }

    async function editDevice(id) {
        var device = devices.find(function (d) { return d.id === id; });
        if (!device) return;

        var name = prompt('Device name:', device.name);
        if (name === null) return;
        var mac = prompt('MAC address:', device.mac_address);
        if (mac === null) return;
        var ip = prompt('IP address:', device.ip_address);
        if (ip === null) return;
        var port = prompt('WoL port:', device.port);
        if (port === null) return;

        var res = await api('/api/devices/' + id, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                name: name.trim(),
                mac_address: mac.trim(),
                ip_address: ip.trim(),
                port: parseInt(port, 10) || 9,
            }),
        });

        if (!res) return;
        if (!res.ok) {
            var data = await res.json();
            alert(data.error || 'Failed to update device');
            return;
        }

        await loadDevices();
    }

    async function deleteDevice(id) {
        if (!confirm('Delete this device?')) return;

        var res = await api('/api/devices/' + id, { method: 'DELETE' });
        if (!res) return;
        if (!res.ok) {
            var data = await res.json();
            alert(data.error || 'Failed to delete device');
            return;
        }

        await loadDevices();
    }

    // --- Wake & Status ---

    async function wakeDevice(id) {
        updateStatusIndicator(id, 'waking');

        var res = await api('/api/devices/' + id + '/wake', { method: 'POST' });
        if (!res) return;
        if (!res.ok) {
            var data = await res.json();
            alert(data.error || 'Failed to send WoL packet');
            updateStatusIndicator(id, 'offline');
            return;
        }

        pollStatus(id);
    }

    async function checkStatus(id) {
        var res = await api('/api/devices/' + id + '/status');
        if (!res) return 'offline';
        if (!res.ok) return 'offline';
        var data = await res.json();
        return data.status || 'offline';
    }

    function pollStatus(id) {
        var interval = 3000;
        var maxDuration = 60000;
        var startTime = Date.now();

        updateStatusIndicator(id, 'waking');

        var timer = setInterval(async function () {
            if (Date.now() - startTime >= maxDuration) {
                clearInterval(timer);
                var finalStatus = await checkStatus(id);
                updateStatusIndicator(id, finalStatus);
                return;
            }

            var status = await checkStatus(id);
            if (status === 'online') {
                clearInterval(timer);
                updateStatusIndicator(id, 'online');
            }
        }, interval);
    }

    // --- Network Scan ---

    async function scanNetwork() {
        var container = document.getElementById('scan-results');
        container.hidden = false;
        container.textContent = '';

        var loadingDiv = document.createElement('div');
        loadingDiv.className = 'scan-item';
        var loadingInfo = document.createElement('div');
        loadingInfo.className = 'scan-item-info';
        var spinnerSpan = document.createElement('span');
        spinnerSpan.className = 'spinner';
        loadingInfo.appendChild(spinnerSpan);
        loadingInfo.appendChild(document.createTextNode('Scanning network...'));
        loadingDiv.appendChild(loadingInfo);
        container.appendChild(loadingDiv);

        var res = await api('/api/network/scan', { method: 'POST' });
        if (!res) return;
        if (!res.ok) {
            container.textContent = '';
            var errDiv = document.createElement('div');
            errDiv.className = 'scan-item';
            var errSpan = document.createElement('span');
            errSpan.className = 'scan-item-info';
            errSpan.style.color = '#e94560';
            errSpan.textContent = 'Scan failed.';
            errDiv.appendChild(errSpan);
            container.appendChild(errDiv);
            return;
        }

        var entries = await res.json();
        if (!Array.isArray(entries) || entries.length === 0) {
            container.textContent = '';
            var emptyDiv = document.createElement('div');
            emptyDiv.className = 'scan-item';
            var emptySpan = document.createElement('span');
            emptySpan.className = 'scan-item-info';
            emptySpan.textContent = 'No devices found on the network.';
            emptyDiv.appendChild(emptySpan);
            container.appendChild(emptyDiv);
            return;
        }

        container.textContent = '';
        entries.forEach(function (entry) {
            var itemDiv = document.createElement('div');
            itemDiv.className = 'scan-item';

            var infoDiv = document.createElement('div');
            var ipSpan = document.createElement('span');
            ipSpan.className = 'scan-item-info';
            ipSpan.textContent = entry.ip;
            infoDiv.appendChild(ipSpan);

            infoDiv.appendChild(document.createTextNode(' '));

            var macSpan = document.createElement('span');
            macSpan.className = 'scan-item-mac';
            macSpan.textContent = entry.mac;
            infoDiv.appendChild(macSpan);

            itemDiv.appendChild(infoDiv);

            var addBtn = document.createElement('button');
            addBtn.className = 'btn btn-secondary btn-sm';
            addBtn.textContent = 'Add';
            addBtn.addEventListener('click', function () {
                addScannedDevice(entry.mac, entry.ip);
            });
            itemDiv.appendChild(addBtn);

            container.appendChild(itemDiv);
        });
    }

    function addScannedDevice(mac, ip) {
        document.getElementById('add-mac').value = mac;
        document.getElementById('add-ip').value = ip;
        document.getElementById('add-form').hidden = false;
        document.getElementById('add-name').focus();
    }

    // --- Auth ---

    async function logout() {
        await api('/api/logout', { method: 'POST' });
        window.location.href = '/login';
    }

    // --- Init ---

    // Expose functions to inline onclick handlers
    window.toggleAddForm = toggleAddForm;
    window.addDevice = addDevice;
    window.editDevice = editDevice;
    window.deleteDevice = deleteDevice;
    window.wakeDevice = wakeDevice;
    window.scanNetwork = scanNetwork;
    window.addScannedDevice = addScannedDevice;
    window.logout = logout;

    document.addEventListener('DOMContentLoaded', loadDevices);
})();

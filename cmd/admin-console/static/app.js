/* Simple SPA-like behavior with vanilla JS.
 * - Tabs switch between views
 * - Fetch devices and manifests from the Go API
 * - Upload manifests via multipart/form-data
 */

function $(sel) { return document.querySelector(sel); }
function $all(sel) { return document.querySelectorAll(sel); }

const btnDevices = $('#btn-devices');
const btnManifests = $('#btn-manifests');
const btnUpload = $('#btn-upload');

const viewDevices = $('#view-devices');
const viewManifests = $('#view-manifests');
const viewUpload = $('#view-upload');
const deviceDetailPanel = $('#device-detail-panel');
const deviceDetailTitle = $('#device-detail-title');
const deviceDetailBody = $('#device-detail-body');
let selectedDeviceKID = null;

function setActive(btn, view) {
  $all('.nav-btn').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  $all('.view').forEach(v => v.classList.remove('visible'));
  view.classList.add('visible');
}

btnDevices.addEventListener('click', () => { setActive(btnDevices, viewDevices); loadDevices(); });
btnManifests.addEventListener('click', () => { setActive(btnManifests, viewManifests); loadManifests(); });
btnUpload.addEventListener('click', () => { setActive(btnUpload, viewUpload); });

// Initial load
loadDevices();

async function loadDevices() {
  const tbody = document.getElementById('devices-body');
  tbody.innerHTML = '';
  selectedDeviceKID = null;
  if (deviceDetailPanel) deviceDetailPanel.classList.remove('visible');
  if (deviceDetailTitle) deviceDetailTitle.textContent = 'Device Details';
  if (deviceDetailBody) deviceDetailBody.innerHTML = '';
  try {
    const res = await fetch('/api/devices');
    const data = await res.json();
    if (Array.isArray(data)) {
      data.forEach(d => {
        const kid = d.kid || d.KID || '-';
        const lastUpdate = d.last_update || d.lastUpdate || d.updated_at || d.updatedAt || '-';
        const wappList = d.wapp_list || d.WappList || [];

        const tr = document.createElement('tr');
        const tdDevice = document.createElement('td');
        const tdLastUpdate = document.createElement('td');

        tdDevice.textContent = kid;
        tdDevice.className = 'device-clickable';
        tdLastUpdate.textContent = lastUpdate;

        tr.append(tdDevice, tdLastUpdate);
        tbody.appendChild(tr);

        tdDevice.addEventListener('click', () => {
          // Toggle off when clicking the same selected device.
          if (selectedDeviceKID === kid && tr.classList.contains('selected')) {
            selectedDeviceKID = null;
            tr.classList.remove('selected');
            if (deviceDetailPanel) deviceDetailPanel.classList.remove('visible');
            if (deviceDetailTitle) deviceDetailTitle.textContent = 'Device Details';
            if (deviceDetailBody) deviceDetailBody.innerHTML = '';
            return;
          }

          selectedDeviceKID = kid;
          $all('#devices-body tr').forEach(row => row.classList.remove('selected'));
          tr.classList.add('selected');

          if (deviceDetailTitle) {
            deviceDetailTitle.textContent = `Device: ${kid}`;
          }
          if (deviceDetailBody) {
            deviceDetailBody.innerHTML = '';
            if (Array.isArray(wappList) && wappList.length > 0) {
              wappList.forEach(w => {
                const detailTr = document.createElement('tr');
                const nameTd = document.createElement('td');
                const verTd = document.createElement('td');
                nameTd.textContent = w.name || w.Name || '-';
                verTd.textContent = (w.ver !== undefined && w.ver !== null) ? String(w.ver) : '-';
                detailTr.append(nameTd, verTd);
                deviceDetailBody.appendChild(detailTr);
              });
            } else {
              const detailTr = document.createElement('tr');
              const nameTd = document.createElement('td');
              const verTd = document.createElement('td');
              nameTd.textContent = '-';
              verTd.textContent = '-';
              detailTr.append(nameTd, verTd);
              deviceDetailBody.appendChild(detailTr);
            }
          }
          if (deviceDetailPanel) {
            deviceDetailPanel.classList.add('visible');
          }
        });
      });
    }
  } catch (e) {
    console.error('devices fetch failed', e);
  }
}

async function loadManifests() {
  const tbody = document.getElementById('manifests-body');
  tbody.innerHTML = '';
  try {
    const res = await fetch('/api/manifests');
    const data = await res.json();
    if (Array.isArray(data)) {
      data.forEach(m => {
        const tr = document.createElement('tr');
        const td1 = document.createElement('td'); td1.textContent = m.name;
        const td2 = document.createElement('td'); td2.textContent = m.version || '-';
        tr.append(td1, td2);
        tbody.appendChild(tr);
      });
    }
  } catch (e) {
    console.error('manifests fetch failed', e);
  }
}

// Upload form handler
const form = document.getElementById('upload-form');
const statusEl = document.getElementById('upload-status');
form.addEventListener('submit', async (e) => {
  e.preventDefault();
  const fd = new FormData(form);
  statusEl.textContent = 'Uploading...';
  try {
    const res = await fetch('/api/manifests', { method: 'POST', body: fd });
    if (!res.ok) throw new Error(await res.text());
    await res.json();
    statusEl.textContent = 'Upload complete.';
    form.reset();
    loadManifests();
  } catch (err) {
    statusEl.textContent = 'Upload failed: ' + err.message;
  }
});

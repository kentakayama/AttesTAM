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
  try {
    const res = await fetch('/api/devices');
    const data = await res.json();
    if (Array.isArray(data)) {
      data.forEach(d => {
        const tr = document.createElement('tr');
        const kid = d.kid || d.KID || '-';
        const wappList = d.wapp_list || d.WappList || [];
        let name = '-';
        let ver = '-';
        if (Array.isArray(wappList) && wappList.length > 0) {
          const w = wappList[0];
          name = w.name || w.Name || '-';
          ver = (w.ver !== undefined && w.ver !== null) ? String(w.ver) : '-';
        }
        const td1 = document.createElement('td'); td1.textContent = kid;
        const td2 = document.createElement('td'); td2.textContent = name;
        const td3 = document.createElement('td'); td3.textContent = ver;
        tr.append(td1, td2, td3);
        tbody.appendChild(tr);
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
    setActive(btnManifests, viewManifests);
  } catch (err) {
    statusEl.textContent = 'Upload failed: ' + err.message;
  }
});

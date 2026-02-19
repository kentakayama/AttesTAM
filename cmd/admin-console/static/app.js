/* Simple SPA-like behavior with vanilla JS.
 * - Tabs switch between views
 * - Fetch agents and manifests from the Go API
 * - Upload manifests via multipart/form-data
 */

function $(sel) { return document.querySelector(sel); }
function $all(sel) { return document.querySelectorAll(sel); }

const btnAgents = $('#btn-agents');
const btnManifests = $('#btn-manifests');
const btnUpload = $('#btn-upload');

const viewAgents = $('#view-agents');
const viewManifests = $('#view-manifests');
const viewUpload = $('#view-upload');
const agentDetailPanel = $('#agent-detail-panel');
const agentDetailTitle = $('#agent-detail-title');
const agentDetailBody = $('#agent-detail-body');
let selectedAgentKID = null;

function setActive(btn, view) {
  $all('.nav-btn').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  $all('.view').forEach(v => v.classList.remove('visible'));
  view.classList.add('visible');
}

btnAgents.addEventListener('click', () => { setActive(btnAgents, viewAgents); loadAgents(); });
btnManifests.addEventListener('click', () => { setActive(btnManifests, viewManifests); loadManifests(); });
btnUpload.addEventListener('click', () => { setActive(btnUpload, viewUpload); });

// Initial load
loadAgents();

async function loadAgents() {
  const tbody = document.getElementById('agents-body');
  tbody.innerHTML = '';
  selectedAgentKID = null;
  if (agentDetailPanel) {
    agentDetailPanel.classList.remove('visible');
    agentDetailPanel.hidden = true;
  }
  if (agentDetailTitle) agentDetailTitle.textContent = 'Agent Details';
  if (agentDetailBody) agentDetailBody.innerHTML = '';
  try {
    const res = await fetch('/api/agents');
    const data = await res.json();
    if (Array.isArray(data)) {
      data.forEach(agent => {
        const kid = agent.kid || agent.KID || '-';
        const lastUpdate = agent.last_update || agent.lastUpdate || agent.updated_at || agent.updatedAt || '-';
        const installedTCList = agent["installed-tc"] || agent.InstalledTCList || [];

        const tr = document.createElement('tr');
        const tdAgent = document.createElement('td');
        const tdLastUpdate = document.createElement('td');

        tdAgent.textContent = kid;
        tdAgent.className = 'agent-clickable';
        tdLastUpdate.textContent = lastUpdate;

        tr.append(tdAgent, tdLastUpdate);
        tbody.appendChild(tr);

        tdAgent.addEventListener('click', () => {
          // Toggle off when clicking the same selected agent.
          if (selectedAgentKID === kid && tr.classList.contains('selected')) {
            selectedAgentKID = null;
            tr.classList.remove('selected');
            if (agentDetailPanel) {
              agentDetailPanel.classList.remove('visible');
              agentDetailPanel.hidden = true;
            }
            if (agentDetailTitle) agentDetailTitle.textContent = 'Agent Details';
            if (agentDetailBody) agentDetailBody.innerHTML = '';
            return;
          }

          selectedAgentKID = kid;
          $all('#agents-body tr').forEach(row => row.classList.remove('selected'));
          tr.classList.add('selected');

          if (agentDetailTitle) {
            agentDetailTitle.textContent = `Agent: ${kid}`;
          }
          if (agentDetailBody) {
            agentDetailBody.innerHTML = '';
            if (Array.isArray(installedTCList) && installedTCList.length > 0) {
              installedTCList.forEach(installedTC => {
                const detailTr = document.createElement('tr');
                const nameTd = document.createElement('td');
                const verTd = document.createElement('td');
                nameTd.textContent = installedTC.name || installedTC.Name || '-';
                verTd.textContent = (installedTC.version !== undefined && installedTC.version !== null) ? String(installedTC.version) : '-';
                detailTr.append(nameTd, verTd);
                agentDetailBody.appendChild(detailTr);
              });
            } else {
              const detailTr = document.createElement('tr');
              const nameTd = document.createElement('td');
              const verTd = document.createElement('td');
              nameTd.textContent = '-';
              verTd.textContent = '-';
              detailTr.append(nameTd, verTd);
              agentDetailBody.appendChild(detailTr);
            }
          }
          if (agentDetailPanel) {
            agentDetailPanel.hidden = false;
            agentDetailPanel.classList.add('visible');
          }
        });
      });
    }
  } catch (e) {
    console.error('agents fetch failed', e);
  }
}

async function loadManifests() {
  const tbody = document.getElementById('manifests-body');
  tbody.innerHTML = '';
  try {
    const res = await fetch('/api/manifests/service');
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
    const res = await fetch('/api/manifests/register', { method: 'POST', body: fd });
    if (!res.ok) throw new Error(await res.text());
    const disposition = res.headers.get('Content-Disposition') || '';
    if (disposition.includes('attachment')) {
      const blob = await res.blob();
      const filenameMatch = disposition.match(/filename=\"?([^\";]+)\"?/i);
      const filename = filenameMatch ? filenameMatch[1] : 'download.bin';
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(url);
      statusEl.textContent = 'Upload complete. Download started.';
    } else {
      const ct = res.headers.get('Content-Type') || '';
      if (ct.includes('application/json')) {
        await res.json();
      } else {
        await res.text();
      }
      statusEl.textContent = 'Upload complete.';
    }
    form.reset();
    loadManifests();
  } catch (err) {
    statusEl.textContent = 'Upload failed: ' + err.message;
  }
});

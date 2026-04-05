document.addEventListener('DOMContentLoaded', () => {
    
    const ADMIN_TOKEN = 'secret-admin-token'; // Keep same as default env inside Go
    const API_BASE = 'http://localhost:8080';

    // Elements
    const checkForm = document.getElementById('check-form');
    const configForm = document.getElementById('config-form');
    const ruleList = document.getElementById('rule-list');
    
    // Status visualizers
    const resStatus = document.getElementById('res-status');
    const resRemaining = document.getElementById('res-remaining');
    const resRetry = document.getElementById('res-retry');

    // Boot: Fetch Rules
    fetchRules();

    // Event: Fire Rate Limit Check
    checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const payload = {
            endpoint: document.getElementById('c-endpoint').value,
            user_id: document.getElementById('c-userid').value,
            ip: document.getElementById('c-ip').value
        };

        try {
            const res = await fetch(`${API_BASE}/check`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });

            const data = await res.json();
            
            // Update UI
            if (data.allowed) {
                resStatus.textContent = 'ALLOWED';
                resStatus.className = 'value status-allowed';
            } else {
                resStatus.textContent = 'BLOCKED (429)';
                resStatus.className = 'value status-blocked';
            }

            resRemaining.textContent = data.remaining;
            resRetry.textContent = data.retry_after > 0 ? `${data.retry_after.toFixed(2)}s` : '--';
            
            // Re-fetch config to show sliding changes if applicable
            fetchRules();
            
        } catch (err) {
            alert('Failed to reach backend cluster. Is the Go server running?');
            console.error(err);
        }
    });

    // Event: Submit new Rule Configuration
    configForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        const rulePayload = {
            id: document.getElementById('r-id').value,
            dimension: document.getElementById('r-dim').value,
            match: document.getElementById('r-match').value,
            strategy: document.getElementById('r-strat').value,
            rate: parseInt(document.getElementById('r-rate').value),
            period: parseInt(document.getElementById('r-period').value)
        };

        try {
            const res = await fetch(`${API_BASE}/config`, {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${ADMIN_TOKEN}`
                },
                body: JSON.stringify(rulePayload)
            });

            if (res.ok) {
                // Clear form
                document.getElementById('r-id').value = '';
                document.getElementById('r-match').value = '';
                // Reload list
                fetchRules();
            } else {
                const error = await res.json();
                alert(`Error: ${error.error}`);
            }

        } catch (err) {
            console.error(err);
        }
    });

    // Function: Populate Active Rules from Server
    async function fetchRules() {
        try {
            const res = await fetch(`${API_BASE}/config`, {
                headers: { 'Authorization': `Bearer ${ADMIN_TOKEN}` }
            });
            const rulesMap = await res.json();

            ruleList.innerHTML = ''; // wipe

            if (Object.keys(rulesMap).length === 0) {
                ruleList.innerHTML = '<div class="admin-notice">No active rules deployed yet.</div>';
                return;
            }

            for (const [id, rule] of Object.entries(rulesMap)) {
                const div = document.createElement('div');
                div.className = 'rule-item';
                div.innerHTML = `
                    <div class="rule-header">
                        <span class="rule-id">${rule.id}</span>
                        <div style="display:flex; gap:0.5rem; align-items:center;">
                            <span class="rule-strat">${rule.strategy}</span>
                            <button class="delete-btn" onclick="deleteRule('${rule.id}')" title="Delete Rule">✖</button>
                        </div>
                    </div>
                    <div class="rule-details">
                        <span><span class="dim-tag">Dimension:</span> ${rule.dimension}</span>
                        <span><span class="dim-tag">Match:</span> ${rule.match}</span>
                        <span><span class="dim-tag">Rate:</span> ${rule.rate} / ${rule.period}s</span>
                    </div>
                `;
                ruleList.appendChild(div);
            }
        } catch (err) {
            ruleList.innerHTML = '<div class="admin-notice status-blocked">Connection Lost</div>';
        }
    }

    // Global space function for delete button
    window.deleteRule = async function(id) {
        if (!confirm(`Delete rule '${id}'?`)) return;
        
        try {
            await fetch(`${API_BASE}/config/${id}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${ADMIN_TOKEN}` }
            });
            fetchRules();
        } catch (err) {
            console.error(err);
        }
    }

});

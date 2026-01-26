// State
let results = [];
let selectedIndex = -1;
let selectedChannelUrl = '';  // Track by URL instead of index
let scanning = false;
let currentStreamUrl = '';
let hls = null;
let favorites = JSON.parse(localStorage.getItem('m3u-scanner-favorites') || '[]');

// User Agents
const userAgents = {
    '': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'okhttp': 'okhttp/3.10.0',
    'okhttp-mod': 'okHttp/Mod-1.5.0.0',
    'android': 'Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36',
    'ios': 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
    'windows': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
};

// Elements
const fileInput = document.getElementById('fileInput');
const urlBtn = document.getElementById('urlBtn');
const scanBtn = document.getElementById('scanBtn');
const stopBtn = document.getElementById('stopBtn');
const exportBtn = document.getElementById('exportBtn');
const settingsBtn = document.getElementById('settingsBtn');
const filterInput = document.getElementById('filterInput');
const resolutionFilter = document.getElementById('resolutionFilter');
const showAvailable = document.getElementById('showAvailable');
const quickCheck = document.getElementById('quickCheck');
const channelList = document.getElementById('channelList');
const detailPanel = document.getElementById('detailPanel');
const statusText = document.getElementById('statusText');
const progressContainer = document.getElementById('progressContainer');
const progressBar = document.getElementById('progressBar');
const ffprobeStatus = document.getElementById('ffprobeStatus');
const groupFilter = document.getElementById('groupFilter');
const sortBy = document.getElementById('sortBy');

// Initialize
document.addEventListener('DOMContentLoaded', init);

function init() {
    // Initialize theme
    initTheme();
    
    // Check FFprobe
    fetch('/api/status')
        .then(r => r.json())
        .then(data => {
            const status = data.ffprobe ? 'FFprobe: 已安装' : 'FFprobe: 未找到';
            ffprobeStatus.textContent = status;
            ffprobeStatus.style.color = data.ffprobe ? 'var(--success)' : 'var(--warning)';
            if (data.ffprobePath) {
                ffprobeStatus.title = `路径: ${data.ffprobePath}`;
            }
        });

    // Load settings
    fetch('/api/settings')
        .then(r => r.json())
        .then(data => {
            document.getElementById('concurrencyInput').value = data.concurrency;
            document.getElementById('timeoutInput').value = data.timeout;
            quickCheck.checked = data.quickCheck;
            if (data.userAgent) {
                const preset = Object.keys(userAgents).find(key => userAgents[key] === data.userAgent);
                if (preset) {
                    document.getElementById('uaPreset').value = preset;
                } else {
                    document.getElementById('uaPreset').value = 'custom';
                    document.getElementById('customUa').value = data.userAgent;
                    document.getElementById('customUaGroup').style.display = 'block';
                }
            }
        });

    // SSE for real-time updates
    const evtSource = new EventSource('/api/events');
    evtSource.onmessage = (e) => {
        if (e.data === 'progress' || e.data === 'scan_complete') {
            refreshResults();
        }
    };

    // Event listeners
    fileInput.addEventListener('change', handleFileUpload);
    urlBtn.addEventListener('click', () => showModal('urlModal'));
    document.getElementById('loadUrlBtn').addEventListener('click', handleLoadURL);
    scanBtn.addEventListener('click', startScan);
    stopBtn.addEventListener('click', stopScan);
    exportBtn.addEventListener('click', exportResults);
    settingsBtn.addEventListener('click', () => showModal('settingsModal'));
    document.getElementById('saveSettingsBtn').addEventListener('click', saveSettings);
    filterInput.addEventListener('input', renderChannelList);
    showAvailable.addEventListener('change', renderChannelList);
    quickCheck.addEventListener('change', updateQuickCheck);

    // Enter key for URL modal
    document.getElementById('urlInput').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') handleLoadURL();
    });

    // Close modals on background click
    document.querySelectorAll('.modal').forEach(modal => {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) closeModal(modal.id);
        });
    });
}

function updateQuickCheck() {
    fetch('/api/settings')
        .then(r => r.json())
        .then(settings => {
            settings.quickCheck = quickCheck.checked;
            return fetch('/api/settings', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(settings)
            });
        });
}

async function handleFileUpload(e) {
    const file = e.target.files[0];
    if (!file) return;

    const formData = new FormData();
    formData.append('file', file);

    statusText.textContent = '加载中...';
    try {
        const res = await fetch('/api/upload', { method: 'POST', body: formData });
        const data = await res.json();
        if (data.error) throw new Error(data.error);
        statusText.textContent = `已加载 ${data.channels} 个频道`;
        refreshResults();
        scanBtn.disabled = false;
    } catch (err) {
        statusText.textContent = '错误: ' + err.message;
    }
    fileInput.value = '';
}

async function handleLoadURL() {
    const url = document.getElementById('urlInput').value.trim();
    if (!url) return;

    closeModal('urlModal');
    statusText.textContent = '加载播放列表...';

    try {
        const res = await fetch('/api/load-url', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        const data = await res.json();
        if (data.error) throw new Error(data.error);
        statusText.textContent = `已加载 ${data.channels} 个频道`;
        refreshResults();
        scanBtn.disabled = false;
    } catch (err) {
        statusText.textContent = '错误: ' + err.message;
    }
    document.getElementById('urlInput').value = '';
}

async function startScan() {
    try {
        const res = await fetch('/api/scan/start', { method: 'POST' });
        const data = await res.json();
        if (data.error) throw new Error(data.error);
        scanning = true;
        scanBtn.disabled = true;
        stopBtn.disabled = false;
        exportBtn.disabled = true;
        progressContainer.style.display = 'block';
    } catch (err) {
        statusText.textContent = '错误: ' + err.message;
    }
}

async function stopScan() {
    stopBtn.disabled = true;
    statusText.textContent = '正在停止...';
    try {
        await fetch('/api/scan/stop', { method: 'POST' });
        statusText.textContent = '已停止扫描';
    } catch (err) {
        statusText.textContent = '停止失败: ' + err.message;
    }
}

async function refreshResults() {
    try {
        const res = await fetch('/api/results');
        const data = await res.json();
        results = data.results || [];
        scanning = data.scanning;

        // Restore selection by URL (not index) to handle scan reordering
        if (selectedChannelUrl) {
            const foundIndex = results.findIndex(r => r.channel.url === selectedChannelUrl);
            selectedIndex = foundIndex >= 0 ? foundIndex : -1;
        }

        // Update progress
        if (data.progress && data.progress.total > 0) {
            const pct = (data.progress.completed / data.progress.total) * 100;
            progressBar.style.width = pct + '%';
            statusText.textContent = `扫描中: ${data.progress.completed}/${data.progress.total} (可用: ${data.progress.available}, 失败: ${data.progress.failed})`;
        }

        if (!scanning && data.progress && data.progress.completed > 0) {
            progressContainer.style.display = 'none';
            scanBtn.disabled = false;
            stopBtn.disabled = true;
            exportBtn.disabled = false;
            const available = results.filter(r => r.stream_info?.available).length;
            statusText.textContent = `完成: ${available}/${results.length} 可用`;
        }

        renderChannelList();
        if (selectedIndex >= 0) {
            renderDetail();
        }
    } catch (err) {
        console.error('刷新失败:', err);
    }
}

function renderChannelList() {
    const filter = filterInput.value.toLowerCase();
    const onlyAvailable = showAvailable.checked;
    const resFilter = resolutionFilter ? resolutionFilter.value : '';
    const groupFilterVal = groupFilter ? groupFilter.value : '';
    const sortByVal = sortBy ? sortBy.value : 'default';

    // 更新分组下拉选项
    updateGroupFilter();

    let filtered = results.filter((r, i) => {
        r._index = i;
        if (filter && !r.channel.name.toLowerCase().includes(filter) && 
            !r.channel.group_title?.toLowerCase().includes(filter)) {
            return false;
        }
        if (onlyAvailable && (!r.stream_info || !r.stream_info.available)) {
            return false;
        }
        // 分组筛选
        if (groupFilterVal && r.channel.group_title !== groupFilterVal) {
            return false;
        }
        // Resolution filter
        if (resFilter) {
            const cat = getResolutionCategory(r);
            if (resFilter === 'unscanned') {
                if (r.stream_info) return false;
            } else if (cat !== resFilter) {
                return false;
            }
        }
        return true;
    });

    // 排序
    filtered = sortResults(filtered, sortByVal);

    channelList.innerHTML = filtered.map(r => {
        const status = getStatus(r);
        const meta = getMeta(r);
        const isSelected = r.channel.url === selectedChannelUrl;
        const resLabel = getResolutionLabel(r);
        const isFav = favorites.includes(r.channel.url);
        const hdrBadge = getHdrBadge(r);
        return `
            <div class="channel-item ${isSelected ? 'selected' : ''}" 
                 onclick="selectChannel(${r._index})" data-index="${r._index}">
                <div class="channel-status ${status}"></div>
                <div class="channel-info">
                    <div class="channel-name">
                        ${isFav ? '<span class="fav-star">⭐</span>' : ''}
                        ${escapeHtml(r.channel.name)}
                        ${hdrBadge}
                    </div>
                    <div class="channel-group">${escapeHtml(r.channel.group_title || '')}${resLabel ? ' • ' + resLabel : ''}</div>
                </div>
                <div class="channel-meta">${meta}</div>
            </div>
        `;
    }).join('');
}

// 更新分组下拉选项
function updateGroupFilter() {
    if (!groupFilter) return;
    
    const groups = [...new Set(results.map(r => r.channel.group_title).filter(g => g))].sort();
    const currentVal = groupFilter.value;
    
    // 只在分组变化时更新
    const existingGroups = [...groupFilter.options].slice(1).map(o => o.value);
    if (JSON.stringify(groups) !== JSON.stringify(existingGroups)) {
        groupFilter.innerHTML = '<option value="">全部分组</option>' + 
            groups.map(g => `<option value="${escapeHtml(g)}">${escapeHtml(g)}</option>`).join('');
        groupFilter.value = currentVal;
    }
}

// 排序结果
function sortResults(arr, sortByVal) {
    // 收藏的总是在前面
    const favSet = new Set(favorites);
    
    const sorted = [...arr].sort((a, b) => {
        // 收藏优先
        const aFav = favSet.has(a.channel.url);
        const bFav = favSet.has(b.channel.url);
        if (aFav && !bFav) return -1;
        if (!aFav && bFav) return 1;
        
        switch (sortByVal) {
            case 'name':
                return a.channel.name.localeCompare(b.channel.name, 'zh-CN');
            case 'response':
                const aTime = a.stream_info?.available ? a.stream_info.response_time : Infinity;
                const bTime = b.stream_info?.available ? b.stream_info.response_time : Infinity;
                return aTime - bTime;
            case 'resolution':
                return getResolutionScore(b) - getResolutionScore(a);
            default:
                return 0;
        }
    });
    return sorted;
}

// 获取分辨率评分（用于排序）
function getResolutionScore(r) {
    if (!r.stream_info?.video_streams?.length) return 0;
    const v = r.stream_info.video_streams[0];
    return v.width * v.height;
}

function getStatus(r) {
    if (!r.stream_info) return 'pending';
    return r.stream_info.available ? 'available' : 'failed';
}

function getMeta(r) {
    if (!r.stream_info) return '待扫描';
    if (r.stream_info.available) {
        return `${Math.round(r.stream_info.response_time / 1000000)}ms`;
    }
    return '不可用';
}

function selectChannel(index) {
    selectedIndex = index;
    if (index >= 0 && index < results.length) {
        selectedChannelUrl = results[index].channel.url;
    } else {
        selectedChannelUrl = '';
    }
    renderChannelList();
    renderDetail();
}

async function renderDetail() {
    if (selectedIndex < 0 || selectedIndex >= results.length) {
        detailPanel.innerHTML = '<div class="empty-state"><p>选择一个频道查看详情</p></div>';
        return;
    }

    const r = results[selectedIndex];
    const ch = r.channel;
    const info = r.stream_info;

    let html = '';

    // Thumbnail + Action Buttons (横向布局)
    if (info && info.available) {
        const isFav = favorites.includes(ch.url);
        html += `<div class="preview-actions-row">
            <div class="preview-container">
                <img id="channelThumbnail" class="detail-thumbnail" src="" style="display:none" alt="缩略图">
                <div id="thumbnailLoading" class="thumbnail-loading">正在生成缩略图...</div>
            </div>
            <div class="action-buttons-vertical">
                <button class="btn primary action-btn" onclick="showPlayerModal()">▶️ 播放</button>
                <button class="btn action-btn" onclick="copyStreamUrl('${escapeHtml(ch.url)}')">📋 复制URL</button>
                <button class="btn action-btn ${isFav ? 'fav-active' : ''}" onclick="toggleFavorite('${escapeHtml(ch.url)}')">${isFav ? '⭐ 已收藏' : '☆ 收藏'}</button>
            </div>
        </div>`;
    }

    html += `
        <div class="detail-section">
            <h3>📺 频道信息</h3>
            <div class="detail-row"><span class="detail-label">名称</span><span class="detail-value">${escapeHtml(ch.name)}</span></div>
            <div class="detail-row"><span class="detail-label">分组</span><span class="detail-value">${escapeHtml(ch.group_title || '-')}</span></div>
            <div class="detail-row"><span class="detail-label">URL</span><span class="detail-value">${escapeHtml(ch.url)}</span></div>
            ${ch.tvg_id ? `<div class="detail-row"><span class="detail-label">TVG ID</span><span class="detail-value">${escapeHtml(ch.tvg_id)}</span></div>` : ''}
            ${ch.logo ? `<div class="detail-row"><span class="detail-label">Logo</span><span class="detail-value"><img src="${ch.logo}" style="max-height:50px;max-width:100px;"></span></div>` : ''}
        </div>
    `;

    if (info) {
        html += `
            <div class="detail-section">
                <h3>📊 流状态</h3>
                <div class="detail-row">
                    <span class="detail-label">状态</span>
                    <span class="detail-value ${info.available ? 'success' : 'danger'}">${info.available ? '✓ 可用' : '✗ 不可用'}</span>
                </div>
                <div class="detail-row"><span class="detail-label">响应时间</span><span class="detail-value">${Math.round(info.response_time / 1000000)}ms</span></div>
                ${info.error ? `<div class="detail-row"><span class="detail-label">错误</span><span class="detail-value danger">${escapeHtml(info.error)}</span></div>` : ''}
            </div>
        `;

        if (info.format) {
            html += `
                <div class="detail-section">
                    <h3>📦 格式</h3>
                    <div class="detail-row"><span class="detail-label">容器</span><span class="detail-value">${info.format.name}</span></div>
                    ${info.format.long_name ? `<div class="detail-row"><span class="detail-label">格式名称</span><span class="detail-value">${info.format.long_name}</span></div>` : ''}
                    ${info.format.bit_rate > 0 ? `<div class="detail-row"><span class="detail-label">总码率</span><span class="detail-value">${Math.round(info.format.bit_rate / 1000)} kbps</span></div>` : ''}
                </div>
            `;
        }

        if (info.video_streams?.length > 0) {
            html += `<div class="detail-section"><h3>🎥 视频流</h3>`;
            info.video_streams.forEach((v, i) => {
                // Determine scan type from field_order
                const fo = v.field_order?.toLowerCase() || '';
                const isInterlaced = fo === 'tt' || fo === 'bb' || fo === 'tb' || fo === 'bt';
                const scanType = isInterlaced ? '隔行 (i)' : '逐行 (p)';
                const scanBadge = isInterlaced ? '1080i' : (v.height >= 1080 ? '1080p' : '');
                
                html += `
                    <div class="stream-card">
                        <div class="stream-card-title">
                            视频流 #${i}
                            <span class="stream-badge">${v.codec.toUpperCase()}</span>
                            ${v.profile ? `<span class="stream-badge">${v.profile}</span>` : ''}
                            ${v.height >= 1080 && scanBadge ? `<span class="stream-badge">${scanBadge}</span>` : ''}
                            ${v.bit_rate_mode ? `<span class="stream-badge ${v.bit_rate_mode.toLowerCase()}">${v.bit_rate_mode}</span>` : ''}
                        </div>
                        <div class="detail-row"><span class="detail-label">分辨率</span><span class="detail-value">${v.width}x${v.height}</span></div>
                        <div class="detail-row"><span class="detail-label">扫描方式</span><span class="detail-value">${scanType}${v.field_order ? ` (${v.field_order})` : ''}</span></div>
                        ${v.frame_rate > 0 ? `<div class="detail-row"><span class="detail-label">帧率</span><span class="detail-value">${v.frame_rate.toFixed(2)} fps</span></div>` : ''}
                        ${v.bit_rate > 0 ? `<div class="detail-row"><span class="detail-label">码率</span><span class="detail-value">${formatBitrate(v.bit_rate)}${v.bit_rate_mode ? ` (${v.bit_rate_mode})` : ''}</span></div>` : ''}
                        ${v.pixel_format ? `<div class="detail-row"><span class="detail-label">像素格式</span><span class="detail-value">${v.pixel_format}</span></div>` : ''}
                    </div>
                `;
            });
            html += `</div>`;
        }

        if (info.audio_streams?.length > 0) {
            html += `<div class="detail-section"><h3>🔊 音频流</h3>`;
            info.audio_streams.forEach((a, i) => {
                html += `
                    <div class="stream-card">
                        <div class="stream-card-title">
                            音频流 #${i}
                            <span class="stream-badge">${a.codec.toUpperCase()}</span>
                            ${a.language ? `<span class="stream-badge">${a.language}</span>` : ''}
                            ${a.bit_rate_mode ? `<span class="stream-badge ${a.bit_rate_mode.toLowerCase()}">${a.bit_rate_mode}</span>` : ''}
                        </div>
                        <div class="detail-row"><span class="detail-label">声道</span><span class="detail-value">${a.channels}${a.channel_layout ? ` (${a.channel_layout})` : ''}</span></div>
                        ${a.sample_rate > 0 ? `<div class="detail-row"><span class="detail-label">采样率</span><span class="detail-value">${a.sample_rate} Hz</span></div>` : ''}
                        ${a.bit_rate > 0 ? `<div class="detail-row"><span class="detail-label">码率</span><span class="detail-value">${formatBitrate(a.bit_rate)}${a.bit_rate_mode ? ` (${a.bit_rate_mode})` : ''}</span></div>` : ''}
                    </div>
                `;
            });
            html += `</div>`;
        }
    }

    // Show single scan button for unscanned or failed channels
    if (!info || !info.available) {
        const btnText = !info ? '🔍 扫描此频道' : '🔄 重新扫描';
        html += `<div class="action-buttons">
            <button class="btn primary action-btn" onclick="scanSingleChannel('${escapeHtml(ch.url)}')">${btnText}</button>
            <button class="btn action-btn" onclick="copyStreamUrl('${escapeHtml(ch.url)}')">📋 复制URL</button>
        </div>`;
    }

    detailPanel.innerHTML = html;

    // Load thumbnail asynchronously
    if (info && info.available) {
        loadThumbnail(ch.url);
    }
}

async function loadThumbnail(url) {
    try {
        const res = await fetch(`/api/thumbnail?url=${encodeURIComponent(url)}`);
        const data = await res.json();
        if (data.thumbnail) {
            const thumb = document.getElementById('channelThumbnail');
            const loading = document.getElementById('thumbnailLoading');
            if (thumb && loading) {
                thumb.src = data.thumbnail;
                thumb.style.display = 'block';
                loading.style.display = 'none';
            }
        }
    } catch (err) {
        const loading = document.getElementById('thumbnailLoading');
        if (loading) {
            loading.textContent = '缩略图生成失败';
            loading.style.color = 'var(--danger)';
        }
    }
}

async function scanSingleChannel(url) {
    statusText.textContent = '正在扫描频道...';
    try {
        const res = await fetch('/api/scan/single', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        const data = await res.json();
        if (data.error) throw new Error(data.error);
        
        // Refresh to get updated results
        await refreshResults();
        statusText.textContent = data.stream_info?.available ? '扫描完成: 可用' : '扫描完成: 不可用';
    } catch (err) {
        statusText.textContent = '扫描失败: ' + err.message;
    }
}

function showPlayerModal() {
    if (selectedIndex < 0) return;
    currentStreamUrl = results[selectedIndex].channel.url;
    showModal('playerModal');
}

async function playWith(player) {
    closeModal('playerModal');
    try {
        const res = await fetch(`/api/play?url=${encodeURIComponent(currentStreamUrl)}&player=${player}`);
        const data = await res.json();
        if (data.error) throw new Error(data.error);
    } catch (err) {
        alert('打开播放器失败: ' + err.message);
    }
}

function playInBrowser() {
    closeModal('playerModal');
    const title = selectedIndex >= 0 ? results[selectedIndex].channel.name : '正在播放';
    document.getElementById('playerTitle').textContent = title;
    
    const video = document.getElementById('webPlayer');
    const isHls = currentStreamUrl.toLowerCase().includes('.m3u8');
    
    // 先尝试直接播放，失败后走代理
    tryPlayDirect(video, currentStreamUrl, isHls);
    showModal('webPlayerModal');
}

// 尝试直接播放，失败后回退到代理
function tryPlayDirect(video, url, isHls) {
    let usedProxy = false;
    
    const playWithUrl = (streamUrl, viaProxy = false) => {
        usedProxy = viaProxy;
        
        if (isHls && Hls.isSupported()) {
            if (hls) {
                hls.destroy();
            }
            hls = new Hls({
                xhrSetup: function(xhr, url) {
                    // 允许跨域
                    xhr.withCredentials = false;
                }
            });
            hls.loadSource(streamUrl);
            hls.attachMedia(video);
            hls.on(Hls.Events.MANIFEST_PARSED, function() {
                video.play().catch(e => console.log("Auto-play prevented:", e));
            });
            hls.on(Hls.Events.ERROR, function (event, data) {
                if (data.fatal) {
                    // 如果是网络错误且还没用代理，尝试代理
                    if (data.type === Hls.ErrorTypes.NETWORK_ERROR && !usedProxy) {
                        console.log("Direct play failed, trying proxy...");
                        hls.destroy();
                        const proxyUrl = `/api/stream?url=${encodeURIComponent(url)}`;
                        playWithUrl(proxyUrl, true);
                    } else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
                        console.log("Media error, try to recover");
                        hls.recoverMediaError();
                    } else {
                        console.log("Cannot recover, destroy hls");
                        hls.destroy();
                    }
                }
            });
        } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
            // Native HLS support (Safari)
            video.src = streamUrl;
            video.onerror = () => {
                if (!usedProxy) {
                    console.log("Direct play failed, trying proxy...");
                    const proxyUrl = `/api/stream?url=${encodeURIComponent(url)}`;
                    playWithUrl(proxyUrl, true);
                }
            };
            video.onloadedmetadata = () => {
                video.play().catch(e => console.log("Auto-play prevented:", e));
            };
        } else {
            // 普通视频格式，直接尝试
            video.src = streamUrl;
            video.onerror = () => {
                if (!usedProxy) {
                    console.log("Direct play failed, trying proxy...");
                    const proxyUrl = `/api/stream?url=${encodeURIComponent(url)}`;
                    playWithUrl(proxyUrl, true);
                }
            };
        }
    };
    
    // 先尝试直接播放
    playWithUrl(url, false);
}

function stopWebPlayer() {
    const video = document.getElementById('webPlayer');
    video.pause();
    video.src = '';
    video.removeAttribute('src');
    if (hls) {
        hls.destroy();
        hls = null;
    }
}

function exportResults() {
    showStatsModal();
}

function showStatsModal() {
    const total = results.length;
    const scanned = results.filter(r => r.stream_info).length;
    const available = results.filter(r => r.stream_info?.available).length;
    const failed = results.filter(r => r.stream_info && !r.stream_info.available).length;
    const unscanned = total - scanned;

    // Resolution distribution
    const resolutions = {};
    const codecs = {};
    
    results.forEach(r => {
        if (r.stream_info?.video_streams?.length > 0) {
            const v = r.stream_info.video_streams[0];
            const resKey = `${v.width}x${v.height}`;
            resolutions[resKey] = (resolutions[resKey] || 0) + 1;
            
            if (v.codec) {
                const codecKey = v.codec.toUpperCase();
                codecs[codecKey] = (codecs[codecKey] || 0) + 1;
            }
        }
    });

    // Sort resolutions by count
    const sortedRes = Object.entries(resolutions)
        .sort((a, b) => b[1] - a[1])
        .slice(0, 10);
    
    const sortedCodecs = Object.entries(codecs)
        .sort((a, b) => b[1] - a[1])
        .slice(0, 5);

    let html = `
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">${total}</div>
                <div class="stat-label">总频道数</div>
            </div>
            <div class="stat-card success">
                <div class="stat-value">${available}</div>
                <div class="stat-label">可用</div>
            </div>
            <div class="stat-card danger">
                <div class="stat-value">${failed}</div>
                <div class="stat-label">不可用</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">${unscanned}</div>
                <div class="stat-label">未扫描</div>
            </div>
        </div>
    `;

    if (sortedRes.length > 0) {
        html += `<div class="stats-section">
            <h4>分辨率分布</h4>
            <div class="stats-bars">
                ${sortedRes.map(([res, count]) => {
                    const pct = (count / available * 100).toFixed(1);
                    return `<div class="stats-bar-row">
                        <span class="stats-bar-label">${res}</span>
                        <div class="stats-bar-container">
                            <div class="stats-bar-fill" style="width: ${pct}%"></div>
                        </div>
                        <span class="stats-bar-value">${count} (${pct}%)</span>
                    </div>`;
                }).join('')}
            </div>
        </div>`;
    }

    if (sortedCodecs.length > 0) {
        html += `<div class="stats-section">
            <h4>视频编码分布</h4>
            <div class="stats-bars">
                ${sortedCodecs.map(([codec, count]) => {
                    const pct = (count / available * 100).toFixed(1);
                    return `<div class="stats-bar-row">
                        <span class="stats-bar-label">${codec}</span>
                        <div class="stats-bar-container">
                            <div class="stats-bar-fill" style="width: ${pct}%"></div>
                        </div>
                        <span class="stats-bar-value">${count} (${pct}%)</span>
                    </div>`;
                }).join('')}
            </div>
        </div>`;
    }

    document.getElementById('statsContent').innerHTML = html;
    showModal('statsModal');
}

function selectUserAgent() {
    const preset = document.getElementById('uaPreset').value;
    const customGroup = document.getElementById('customUaGroup');
    
    if (preset === 'custom') {
        customGroup.style.display = 'block';
    } else {
        customGroup.style.display = 'none';
    }
}

async function saveSettings() {
    const preset = document.getElementById('uaPreset').value;
    let userAgent = userAgents[preset] || userAgents[''];
    
    if (preset === 'custom') {
        userAgent = document.getElementById('customUa').value.trim() || userAgents[''];
    }

    const settings = {
        concurrency: parseInt(document.getElementById('concurrencyInput').value) || 20,
        timeout: parseInt(document.getElementById('timeoutInput').value) || 15,
        quickCheck: quickCheck.checked,
        userAgent: userAgent
    };

    try {
        const res = await fetch('/api/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings)
        });
        const data = await res.json();
        if (data.error) throw new Error(data.error);
        closeModal('settingsModal');
    } catch (err) {
        alert('保存设置失败: ' + err.message);
    }
}

function showModal(id) {
    document.getElementById(id).classList.add('active');
}

function closeModal(id) {
    document.getElementById(id).classList.remove('active');
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// 格式化码率显示
function formatBitrate(bps) {
    if (bps >= 1000000) {
        return (bps / 1000000).toFixed(2) + ' Mbps';
    }
    return Math.round(bps / 1000) + ' kbps';
}

function getResolutionCategory(r) {
    if (!r.stream_info || !r.stream_info.video_streams?.length) {
        return 'unscanned';
    }
    const v = r.stream_info.video_streams[0];
    const h = v.height;
    if (h >= 2160) return '4k';
    if (h >= 1080) {
        // Check field_order for interlaced detection
        // tt=top field first, bb=bottom field first, tb/bt=mixed
        const fo = v.field_order?.toLowerCase() || '';
        const isInterlaced = fo === 'tt' || fo === 'bb' || fo === 'tb' || fo === 'bt';
        return isInterlaced ? '1080i' : '1080p';
    }
    if (h >= 720) return '720p';
    if (h >= 576) return '576';
    if (h >= 480) return '480';
    if (h > 0) return 'other';
    return 'unscanned';
}

function getResolutionLabel(r) {
    if (!r.stream_info || !r.stream_info.video_streams?.length) {
        return '';
    }
    const v = r.stream_info.video_streams[0];
    return `${v.width}x${v.height}`;
}

// 获取HDR/杜比标识
function getHdrBadge(r) {
    if (!r.stream_info?.video_streams?.length) return '';
    
    const v = r.stream_info.video_streams[0];
    const badges = [];
    
    // 检测HDR (通过color_transfer或color_primaries)
    const colorTransfer = v.color_transfer?.toLowerCase() || '';
    const colorPrimaries = v.color_primaries?.toLowerCase() || '';
    const colorSpace = v.color_space?.toLowerCase() || '';
    const profile = v.profile?.toLowerCase() || '';
    
    // HDR10
    if (colorTransfer.includes('smpte2084') || colorTransfer.includes('st2084') || 
        colorPrimaries.includes('bt2020') || profile.includes('main 10')) {
        badges.push('<span class="hdr-badge hdr">HDR</span>');
    }
    
    // 杜比视界 (通过profile检测)
    if (profile.includes('dvhe') || profile.includes('dovi') || profile.includes('dolby')) {
        badges.push('<span class="hdr-badge dv">DV</span>');
    }
    
    // HLG
    if (colorTransfer.includes('hlg') || colorTransfer.includes('arib-std-b67')) {
        badges.push('<span class="hdr-badge hlg">HLG</span>');
    }
    
    // 检测音频 Atmos
    if (r.stream_info?.audio_streams?.length) {
        for (const a of r.stream_info.audio_streams) {
            const codec = a.codec?.toLowerCase() || '';
            const layout = a.channel_layout?.toLowerCase() || '';
            if (codec.includes('truehd') || codec.includes('atmos') || 
                layout.includes('atmos') || a.channels > 6) {
                badges.push('<span class="hdr-badge atmos">Atmos</span>');
                break;
            }
        }
    }
    
    return badges.join('');
}

function initTheme() {
    const savedTheme = localStorage.getItem('m3u-scanner-theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
    updateThemeButton(savedTheme);
}

function toggleTheme() {
    const currentTheme = document.documentElement.getAttribute('data-theme') || 'dark';
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', newTheme);
    localStorage.setItem('m3u-scanner-theme', newTheme);
    updateThemeButton(newTheme);
}

function updateThemeButton(theme) {
    const btn = document.getElementById('themeBtn');
    if (btn) {
        btn.textContent = theme === 'dark' ? '☀️' : '🌙';
        btn.title = theme === 'dark' ? '切换到亮色主题' : '切换到暗色主题';
    }
}

// 复制URL到剪贴板
function copyStreamUrl(url) {
    navigator.clipboard.writeText(url).then(() => {
        showToast('已复制到剪贴板');
    }).catch(err => {
        // Fallback for older browsers
        const textarea = document.createElement('textarea');
        textarea.value = url;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        showToast('已复制到剪贴板');
    });
}

// Toast提示
function showToast(message) {
    let toast = document.getElementById('toast');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'toast';
        document.body.appendChild(toast);
    }
    toast.textContent = message;
    toast.classList.add('show');
    setTimeout(() => toast.classList.remove('show'), 2000);
}

// 收藏/取消收藏
function toggleFavorite(url) {
    const index = favorites.indexOf(url);
    if (index > -1) {
        favorites.splice(index, 1);
        showToast('已取消收藏');
    } else {
        favorites.push(url);
        showToast('已添加收藏');
    }
    localStorage.setItem('m3u-scanner-favorites', JSON.stringify(favorites));
    renderChannelList();
    renderDetail();
}

// 键盘导航
document.addEventListener('keydown', (e) => {
    // 如果在输入框中，不处理
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') {
        return;
    }
    
    const items = document.querySelectorAll('.channel-item');
    if (items.length === 0) return;
    
    switch (e.key) {
        case 'ArrowUp':
            e.preventDefault();
            navigateChannel(-1, items);
            break;
        case 'ArrowDown':
            e.preventDefault();
            navigateChannel(1, items);
            break;
        case 'Enter':
            if (selectedIndex >= 0 && results[selectedIndex]?.stream_info?.available) {
                showPlayerModal();
            }
            break;
        case 'f':
        case 'F':
            if (selectedIndex >= 0) {
                toggleFavorite(results[selectedIndex].channel.url);
            }
            break;
        case 'c':
        case 'C':
            if (selectedIndex >= 0) {
                copyStreamUrl(results[selectedIndex].channel.url);
            }
            break;
    }
});

function navigateChannel(direction, items) {
    // 找到当前选中项在可见列表中的位置
    let currentVisibleIndex = -1;
    items.forEach((item, i) => {
        if (parseInt(item.dataset.index) === selectedIndex) {
            currentVisibleIndex = i;
        }
    });
    
    let newVisibleIndex = currentVisibleIndex + direction;
    if (newVisibleIndex < 0) newVisibleIndex = 0;
    if (newVisibleIndex >= items.length) newVisibleIndex = items.length - 1;
    
    const newIndex = parseInt(items[newVisibleIndex].dataset.index);
    selectChannel(newIndex);
    
    // 滚动到可见区域
    items[newVisibleIndex].scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

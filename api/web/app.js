// EAN-13 条码解码器前端逻辑（含 MCP 商品查询）
(function() {
    'use strict';

    // ========== DOM 元素：条码解码 ==========
    var uploadArea = document.getElementById('uploadArea');
    var fileInput = document.getElementById('fileInput');
    var previewArea = document.getElementById('previewArea');
    var previewImage = document.getElementById('previewImage');
    var btnClear = document.getElementById('btnClear');
    var btnDecode = document.getElementById('btnDecode');
    var resultArea = document.getElementById('resultArea');
    var resultText = document.getElementById('resultText');
    var resultFormat = document.getElementById('resultFormat');
    var btnCopy = document.getElementById('btnCopy');
    var errorArea = document.getElementById('errorArea');
    var errorText = document.getElementById('errorText');

    // ========== DOM 元素：商品卡片 ==========
    var productCard = document.getElementById('productCard');
    var productLoading = document.getElementById('productLoading');
    var productStatus = document.getElementById('productStatus');
    var productStatusText = document.getElementById('productStatusText');
    var productContent = document.getElementById('productContent');
    var productImageWrap = document.getElementById('productImageWrap');
    var productImage = document.getElementById('productImage');
    var productFields = document.getElementById('productFields');
    var productJsonView = document.getElementById('productJsonView');
    var productJsonContent = document.getElementById('productJsonContent');

    // ========== DOM 元素：MCP 设置 ==========
    var mcpPanel = document.getElementById('mcpPanel');
    var mcpPanelHeader = document.getElementById('mcpPanelHeader');
    var mcpToggleIcon = document.getElementById('mcpToggleIcon');
    var mcpPanelBody = document.getElementById('mcpPanelBody');
    var mcpJsonInput = document.getElementById('mcpJsonInput');
    var mcpServerSelectField = document.getElementById('mcpServerSelectField');
    var mcpServerSelect = document.getElementById('mcpServerSelect');
    var mcpUrlDisplay = document.getElementById('mcpUrlDisplay');
    var btnUrlToggle = document.getElementById('btnUrlToggle');
    var btnUrlEdit = document.getElementById('btnUrlEdit');
    var btnMcpTest = document.getElementById('btnMcpTest');
    var btnMcpSave = document.getElementById('btnMcpSave');
    var btnMcpClear = document.getElementById('btnMcpClear');
    var mcpTestResult = document.getElementById('mcpTestResult');
    var mcpTestResultText = document.getElementById('mcpTestResultText');
    var mcpToolSelectField = document.getElementById('mcpToolSelectField');
    var mcpToolSelect = document.getElementById('mcpToolSelect');
    var mcpParamSelectField = document.getElementById('mcpParamSelectField');
    var mcpParamSelect = document.getElementById('mcpParamSelect');

    // ========== 状态变量 ==========
    var currentFile = null;
    var currentObjectURL = null;
    var serverConfig = null;
    var urlMasked = true;          // URL 默认遮罩
    var urlEditing = false;        // URL 编辑模式
    var parsedServers = [];        // 解析后的服务器列表
    var mcpTools = [];             // 测试连接后获取的工具列表
    var currentEAN13 = '';         // 当前识别到的条码

    // ========== localStorage 版本化键名 ==========
    var STORAGE_KEY = 'ean13_mcp_config_v1';

    // ========== 启动 ==========
    fetch('/api/config')
        .then(function(r) { return r.json(); })
        .then(function(cfg) { serverConfig = cfg; })
        .catch(function() {});

    loadSavedConfig();

    // ============================================================
    //  条码解码：原有功能
    // ============================================================

    uploadArea.addEventListener('click', function() {
        fileInput.click();
    });

    fileInput.addEventListener('change', function(e) {
        if (e.target.files && e.target.files[0]) {
            handleFile(e.target.files[0]);
        }
    });

    uploadArea.addEventListener('dragover', function(e) {
        e.preventDefault();
        uploadArea.classList.add('dragover');
    });

    uploadArea.addEventListener('dragleave', function(e) {
        e.preventDefault();
        uploadArea.classList.remove('dragover');
    });

    uploadArea.addEventListener('drop', function(e) {
        e.preventDefault();
        uploadArea.classList.remove('dragover');
        if (e.dataTransfer.files && e.dataTransfer.files[0]) {
            handleFile(e.dataTransfer.files[0]);
        }
    });

    document.addEventListener('paste', function(e) {
        var items = e.clipboardData && e.clipboardData.items;
        if (!items) return;
        for (var i = 0; i < items.length; i++) {
            if (items[i].type.indexOf('image') !== -1) {
                var file = items[i].getAsFile();
                if (file) {
                    handleFile(file);
                    e.preventDefault();
                    break;
                }
            }
        }
    });

    btnClear.addEventListener('click', function(e) {
        e.stopPropagation();
        clearImage();
    });

    btnDecode.addEventListener('click', function() {
        if (currentFile) {
            decodeImage(currentFile);
        }
    });

    btnCopy.addEventListener('click', function() {
        var text = resultText.textContent;
        if (!text) return;
        var textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        try {
            document.execCommand('copy');
            btnCopy.textContent = '已复制';
            btnCopy.classList.add('copied');
            setTimeout(function() {
                btnCopy.textContent = '复制';
                btnCopy.classList.remove('copied');
            }, 2000);
        } catch (err) {
            btnCopy.textContent = '复制失败';
        }
        document.body.removeChild(textarea);
    });

    function handleFile(file) {
        if (!file.type.match(/image\/(jpeg|png)/)) {
            showError('仅支持 PNG 和 JPEG 格式');
            return;
        }
        if (serverConfig && file.size > serverConfig.maxUploadBytes) {
            showError('文件过大（最大 ' + Math.round(serverConfig.maxUploadBytes / 1024 / 1024) + ' MiB）');
            return;
        }
        currentFile = file;
        hideError();
        hideResult();
        hideProductCard();
        if (currentObjectURL) {
            URL.revokeObjectURL(currentObjectURL);
        }
        currentObjectURL = URL.createObjectURL(file);
        previewImage.src = currentObjectURL;
        previewArea.style.display = 'block';
        uploadArea.style.display = 'none';
        btnDecode.disabled = false;
    }

    function clearImage() {
        currentFile = null;
        currentEAN13 = '';
        if (currentObjectURL) {
            URL.revokeObjectURL(currentObjectURL);
            currentObjectURL = null;
        }
        previewImage.src = '';
        previewArea.style.display = 'none';
        uploadArea.style.display = 'block';
        btnDecode.disabled = true;
        fileInput.value = '';
        hideResult();
        hideError();
        hideProductCard();
    }

    function decodeImage(file) {
        btnDecode.disabled = true;
        btnDecode.classList.add('loading');
        hideError();
        hideResult();
        hideProductCard();

        var formData = new FormData();
        formData.append('image', file);

        fetch('/api/decode', {
            method: 'POST',
            body: formData
        })
        .then(function(response) { return response.json(); })
        .then(function(data) {
            btnDecode.classList.remove('loading');
            btnDecode.disabled = false;
            if (data.success) {
                showResult(data.text, data.format);
                currentEAN13 = data.text;
                // 条码识别成功后自动查询商品信息
                autoLookupBarcode(data.text);
            } else {
                showError(data.error || '解码失败，未找到 EAN-13 条码');
            }
        })
        .catch(function(err) {
            btnDecode.classList.remove('loading');
            btnDecode.disabled = false;
            showError('请求失败：' + err.message);
        });
    }

    function showResult(text, format) {
        resultText.textContent = text;
        resultFormat.textContent = format || 'EAN_13';
        resultArea.style.display = 'block';
    }

    function hideResult() {
        resultArea.style.display = 'none';
    }

    function showError(message) {
        errorText.textContent = message;
        errorArea.style.display = 'block';
    }

    function hideError() {
        errorArea.style.display = 'none';
    }

    // ============================================================
    //  MCP 设置面板：折叠/展开
    // ============================================================

    mcpPanelHeader.addEventListener('click', function() {
        var visible = mcpPanelBody.style.display !== 'none';
        mcpPanelBody.style.display = visible ? 'none' : 'block';
        mcpToggleIcon.classList.toggle('open', !visible);
    });

    // ============================================================
    //  MCP JSON 解析与服务器选择
    // ============================================================

    mcpJsonInput.addEventListener('input', function() {
        parseMCPJson();
    });

    function parseMCPJson() {
        var raw = mcpJsonInput.value.trim();
        parsedServers = [];
        mcpServerSelect.innerHTML = '';

        if (!raw) {
            mcpServerSelectField.style.display = 'none';
            updateUrlDisplay();
            return;
        }

        var config;
        try {
            config = JSON.parse(raw);
        } catch (e) {
            mcpServerSelectField.style.display = 'none';
            updateUrlDisplay();
            return;
        }

        // 支持两种格式：
        // 1. { "mcpServers": { "name": { "type": "...", "url": "..." } } }
        // 2. { "name": { "type": "...", "url": "..." } }
        var servers = config.mcpServers || config;
        if (typeof servers !== 'object' || Array.isArray(servers)) {
            mcpServerSelectField.style.display = 'none';
            updateUrlDisplay();
            return;
        }

        for (var name in servers) {
            if (!servers.hasOwnProperty(name)) continue;
            var srv = servers[name];
            if (srv && srv.url) {
                // 支持不同 transport key
                var type = srv.type || 'streamableHttp';
                parsedServers.push({ name: name, type: type, url: srv.url });
            }
        }

        if (parsedServers.length === 0) {
            mcpServerSelectField.style.display = 'none';
            updateUrlDisplay();
            return;
        }

        if (parsedServers.length > 1) {
            mcpServerSelectField.style.display = 'block';
            for (var i = 0; i < parsedServers.length; i++) {
                var opt = document.createElement('option');
                opt.value = i;
                opt.textContent = parsedServers[i].name + ' (' + parsedServers[i].type + ')';
                mcpServerSelect.appendChild(opt);
            }
        } else {
            mcpServerSelectField.style.display = 'none';
        }

        updateUrlDisplay();
    }

    function getSelectedServer() {
        if (parsedServers.length === 0) return null;
        if (parsedServers.length === 1) return parsedServers[0];
        var idx = parseInt(mcpServerSelect.value, 10);
        return parsedServers[idx] || parsedServers[0];
    }

    mcpServerSelect.addEventListener('change', function() {
        updateUrlDisplay();
    });

    function updateUrlDisplay() {
        var srv = getSelectedServer();
        if (srv) {
            if (urlMasked && !urlEditing) {
                mcpUrlDisplay.value = maskUrl(srv.url);
            } else {
                mcpUrlDisplay.value = srv.url;
            }
        } else {
            mcpUrlDisplay.value = '';
        }
    }

    function maskUrl(url) {
        try {
            var u = new URL(url);
            return u.protocol + '//' + u.hostname + ':***/***';
        } catch (e) {
            return '***';
        }
    }

    // ============================================================
    //  URL 遮罩/编辑切换
    // ============================================================

    btnUrlToggle.addEventListener('click', function() {
        urlMasked = !urlMasked;
        updateUrlDisplay();
        btnUrlToggle.textContent = urlMasked ? '👁' : '🙈';
    });

    btnUrlEdit.addEventListener('click', function() {
        if (!urlEditing) {
            urlEditing = true;
            mcpUrlDisplay.readOnly = false;
            mcpUrlDisplay.style.background = '#fffde7';
            mcpUrlDisplay.focus();
            btnUrlEdit.textContent = '✅';
            // 显示完整 URL
            var srv = getSelectedServer();
            if (srv) mcpUrlDisplay.value = srv.url;
        } else {
            urlEditing = false;
            mcpUrlDisplay.readOnly = true;
            mcpUrlDisplay.style.background = '#fff';
            btnUrlEdit.textContent = '✏️';
            // 回写编辑后的 URL
            var srv = getSelectedServer();
            if (srv) {
                srv.url = mcpUrlDisplay.value;
                // 同步回 JSON
                syncUrlToJSON(srv);
            }
            updateUrlDisplay();
        }
    });

    function syncUrlToJSON(srv) {
        // 回写编辑后的 URL 到 textarea 的 JSON 中
        try {
            var raw = mcpJsonInput.value.trim();
            if (!raw) return;
            var config = JSON.parse(raw);
            var servers = config.mcpServers || config;
            if (servers[srv.name]) {
                servers[srv.name].url = srv.url;
                mcpJsonInput.value = JSON.stringify(config, null, 2);
            }
        } catch (e) {
            // ignore JSON sync errors
        }
    }

    // ============================================================
    //  连接测试
    // ============================================================

    btnMcpTest.addEventListener('click', function() {
        var srv = getSelectedServer();
        if (!srv) {
            showMcpTestResult('error', '请先配置 MCP 服务器');
            return;
        }

        btnMcpTest.disabled = true;
        btnMcpTest.textContent = '测试中...';
        showMcpTestResult('', '正在连接...');

        fetch('/api/mcp/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ type: srv.type, url: srv.url })
        })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            btnMcpTest.disabled = false;
            btnMcpTest.textContent = '连接测试';
            if (data.success) {
                mcpTools = data.tools || [];
                showMcpTestResult('success', '连接成功，发现 ' + mcpTools.length + ' 个工具');
                populateToolSelect(mcpTools);
            } else {
                mcpTools = [];
                showMcpTestResult('error', '连接失败：' + (data.error || '未知错误'));
                hideToolParamSelects();
            }
        })
        .catch(function(err) {
            btnMcpTest.disabled = false;
            btnMcpTest.textContent = '连接测试';
            showMcpTestResult('error', '请求失败：' + err.message);
        });
    });

    function showMcpTestResult(type, msg) {
        mcpTestResult.style.display = 'block';
        mcpTestResult.className = 'mcp-test-result' + (type ? ' ' + type : '');
        mcpTestResultText.textContent = msg;
    }

    function hideMcpTestResult() {
        mcpTestResult.style.display = 'none';
    }

    // ============================================================
    //  工具/参数选择
    // ============================================================

    function populateToolSelect(tools) {
        mcpToolSelect.innerHTML = '';
        if (tools.length === 0) {
            hideToolParamSelects();
            return;
        }
        for (var i = 0; i < tools.length; i++) {
            var opt = document.createElement('option');
            opt.value = tools[i].name;
            opt.textContent = tools[i].name + (tools[i].description ? ' — ' + tools[i].description : '');
            mcpToolSelect.appendChild(opt);
        }
        mcpToolSelectField.style.display = 'block';
        populateParamSelect(tools[0]);
    }

    mcpToolSelect.addEventListener('change', function() {
        var selectedName = mcpToolSelect.value;
        for (var i = 0; i < mcpTools.length; i++) {
            if (mcpTools[i].name === selectedName) {
                populateParamSelect(mcpTools[i]);
                return;
            }
        }
    });

    function populateParamSelect(tool) {
        mcpParamSelect.innerHTML = '';
        var props = extractProperties(tool);
        if (props.length === 0) {
            mcpParamSelectField.style.display = 'none';
            return;
        }
        for (var i = 0; i < props.length; i++) {
            var opt = document.createElement('option');
            opt.value = props[i];
            opt.textContent = props[i];
            mcpParamSelect.appendChild(opt);
        }
        mcpParamSelectField.style.display = 'block';
    }

    function extractProperties(tool) {
        if (!tool.inputSchema || !tool.inputSchema.properties) return [];
        var props = [];
        for (var key in tool.inputSchema.properties) {
            if (tool.inputSchema.properties.hasOwnProperty(key)) {
                props.push(key);
            }
        }
        return props;
    }

    function hideToolParamSelects() {
        mcpToolSelectField.style.display = 'none';
        mcpParamSelectField.style.display = 'none';
    }

    // ============================================================
    //  保存 / 清除配置
    // ============================================================

    btnMcpSave.addEventListener('click', function() {
        var srv = getSelectedServer();
        if (!srv) {
            showMcpTestResult('error', '请先配置 MCP 服务器');
            return;
        }
        var saveData = {
            json: mcpJsonInput.value,
            selectedServer: parsedServers.indexOf(srv),
            selectedTool: mcpToolSelect.value || '',
            selectedParam: mcpParamSelect.value || ''
        };
        try {
            localStorage.setItem(STORAGE_KEY, JSON.stringify(saveData));
            showMcpTestResult('success', '配置已保存到浏览器');
        } catch (e) {
            showMcpTestResult('error', '保存失败：' + e.message);
        }
    });

    btnMcpClear.addEventListener('click', function() {
        mcpJsonInput.value = '';
        parsedServers = [];
        mcpTools = [];
        currentEAN13 = '';
        mcpServerSelect.innerHTML = '';
        mcpServerSelectField.style.display = 'none';
        mcpUrlDisplay.value = '';
        urlMasked = true;
        urlEditing = false;
        mcpUrlDisplay.readOnly = true;
        mcpUrlDisplay.style.background = '#fff';
        btnUrlEdit.textContent = '✏️';
        btnUrlToggle.textContent = '👁';
        hideMcpTestResult();
        hideToolParamSelects();
        hideProductCard();
        try {
            localStorage.removeItem(STORAGE_KEY);
        } catch (e) {}
    });

    function loadSavedConfig() {
        try {
            var saved = localStorage.getItem(STORAGE_KEY);
            if (!saved) return;
            var data = JSON.parse(saved);
            if (data.json) {
                mcpJsonInput.value = data.json;
                parseMCPJson();
                if (data.selectedServer !== undefined && parsedServers.length > data.selectedServer) {
                    mcpServerSelect.value = data.selectedServer;
                    updateUrlDisplay();
                }
                // 如果有保存的工具名，测试连接以恢复
                if (data.selectedTool) {
                    // 恢复工具选择（需要先连接测试）
                    setTimeout(function() {
                        btnMcpTest.click();
                        // 延迟设置工具和参数选择
                        var checkTools = setInterval(function() {
                            if (mcpTools.length > 0) {
                                clearInterval(checkTools);
                                mcpToolSelect.value = data.selectedTool;
                                // 触发 change 事件以更新参数列表
                                var evt = document.createEvent('HTMLEvents');
                                evt.initEvent('change', true, false);
                                mcpToolSelect.dispatchEvent(evt);
                                if (data.selectedParam) {
                                    setTimeout(function() {
                                        mcpParamSelect.value = data.selectedParam;
                                    }, 100);
                                }
                            }
                        }, 200);
                        // 5 秒后停止检查
                        setTimeout(function() { clearInterval(checkTools); }, 5000);
                    }, 300);
                }
            }
        } catch (e) {
            // ignore load errors
        }
    }

    // ============================================================
    //  条码查询：自动触发
    // ============================================================

    function autoLookupBarcode(ean13) {
        var srv = getSelectedServer();
        if (!srv) return; // 未配置 MCP，跳过

        showProductLoading();

        var body = {
            ean13: ean13,
            mcp: { type: srv.type, url: srv.url }
        };
        // 如果用户已选择工具/参数，传给后端
        if (mcpToolSelect.value) {
            body.toolName = mcpToolSelect.value;
        }
        if (mcpParamSelect.value) {
            body.paramName = mcpParamSelect.value;
        }

        fetch('/api/barcode/lookup', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            if (data.success) {
                showProductResult(data);
            } else if (data.ambiguous && data.availableTools) {
                // 多个匹配工具，更新选择器让用户选择
                mcpTools = data.availableTools;
                populateToolSelect(mcpTools);
                showProductStatus('发现多个匹配工具，请在下方选择后重新解码');
            } else {
                showProductStatus(data.error || '查询失败');
            }
        })
        .catch(function(err) {
            showProductStatus('查询失败：' + err.message);
        });
    }

    // ============================================================
    //  商品卡片渲染
    // ============================================================

    function hideProductCard() {
        productCard.style.display = 'none';
        productLoading.style.display = 'none';
        productStatus.style.display = 'none';
        productContent.style.display = 'none';
        productImageWrap.style.display = 'none';
        productJsonView.style.display = 'none';
        productFields.innerHTML = '';
        productImage.src = '';
        productJsonContent.textContent = '';
    }

    function showProductLoading() {
        productCard.style.display = 'block';
        productLoading.style.display = 'flex';
        productStatus.style.display = 'none';
        productContent.style.display = 'none';
        productJsonView.style.display = 'none';
    }

    function showProductStatus(msg) {
        productCard.style.display = 'block';
        productLoading.style.display = 'none';
        productStatus.style.display = 'block';
        productStatusText.textContent = msg;
        productContent.style.display = 'none';
        productJsonView.style.display = 'none';
    }

    var FIELD_LABELS = {
        name: '名称',
        brand: '品牌',
        manufacturer: '企业',
        category: '类别',
        spec: '规格',
        description: '描述',
        price: '价格',
        manufacturerAddress: '厂商地址',
        barcode: '条码',
        keywords: '关键词',
        imageUrl: '图片'
    };

    function showProductResult(data) {
        productCard.style.display = 'block';
        productLoading.style.display = 'none';
        productStatus.style.display = 'none';

        // 检查是否有结构化字段
        var hasFields = data.name || data.brand || data.manufacturer || data.category ||
                        data.spec || data.description || data.price ||
                        data.manufacturerAddress || data.barcode || data.keywords;

        if (hasFields) {
            productContent.style.display = 'flex';
            productFields.innerHTML = '';

            var fieldKeys = ['name', 'brand', 'manufacturer', 'category', 'spec', 'description', 'price', 'manufacturerAddress', 'barcode', 'keywords'];
            for (var i = 0; i < fieldKeys.length; i++) {
                var key = fieldKeys[i];
                var val = data[key];
                if (!val) continue;
                var item = document.createElement('div');
                item.className = 'product-field-item';
                var label = document.createElement('label');
                label.textContent = FIELD_LABELS[key] || key;
                var valueDiv = document.createElement('div');
                valueDiv.className = 'product-field-value';
                valueDiv.textContent = val;
                item.appendChild(label);
                item.appendChild(valueDiv);
                productFields.appendChild(item);
            }

            // 图片（安全校验：仅接受 HTTP(S) URL）
            if (data.imageUrl && /^https?:\/\//.test(data.imageUrl)) {
                productImageWrap.style.display = 'block';
                productImage.src = data.imageUrl;
            } else {
                productImageWrap.style.display = 'none';
            }

            productJsonView.style.display = 'none';
        } else if (data.rawContent) {
            // 无法解析为常见字段，显示原始 JSON 视图
            productContent.style.display = 'none';
            productJsonView.style.display = 'block';
            productJsonContent.textContent = data.rawContent;
        } else {
            showProductStatus('查询成功但无返回数据');
        }
    }

    // ============================================================
    //  阻止页面拖拽默认行为
    // ============================================================

    document.addEventListener('dragover', function(e) {
        e.preventDefault();
    });

    document.addEventListener('drop', function(e) {
        e.preventDefault();
    });
})();

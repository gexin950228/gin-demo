/* ===== 公共布局交互：navbar 用户菜单 + 头像上传弹窗 ===== */
/* 依赖：jQuery（必须在 jQuery 之后加载） */
/* 头像上传地址由各页面通过 window.AVATAR_UPLOAD_URL 注入 */
(function () {
    'use strict';

    // 当前选中的头像文件（闭包私有，跨函数共享）
    var avatarSelectedFile = null;

    // ===== 用户下拉菜单切换 =====
    // approval_list 没有 .user-dropdown 结构，元素不存在时安全跳过
    window.toggleUserMenu = function () {
        var dropdown = document.querySelector('.user-dropdown');
        if (dropdown) {
            dropdown.classList.toggle('open');
        }
    };

    // ===== 头像上传弹窗 =====
    window.openAvatarModal = function () {
        // 关闭可能打开的用户下拉菜单（approval_list 无该元素时为 no-op）
        $('.user-dropdown').removeClass('open');
        avatarSelectedFile = null;
        $('#avatarFileInput').val('');
        $('#avatarUploadBtn').prop('disabled', true);
        // 预览当前头像
        var $preview = $('#avatarPreview');
        var currentAvatar = $('#navAvatar img').attr('src');
        if (currentAvatar) {
            $preview.html('<img src="' + currentAvatar + '" alt="">');
        } else {
            $preview.html('<div class="avatar-preview-placeholder">点击下方按钮<br>选择图片</div>');
        }
        $('#avatarModalMask').addClass('show');
    };

    window.closeAvatarModal = function () {
        $('#avatarModalMask').removeClass('show');
    };

    // 文件选择 → 校验 + 预览（事件委托，适配 AJAX 注入的弹窗）
    $(document).on('change', '#avatarFileInput', function () {
        var file = this.files[0];
        if (!file) {
            avatarSelectedFile = null;
            $('#avatarUploadBtn').prop('disabled', true);
            return;
        }
        // 简单校验
        if (file.size > 1 * 1024 * 1024) {
            alert('文件不能超过 1MB');
            $(this).val('');
            avatarSelectedFile = null;
            $('#avatarUploadBtn').prop('disabled', true);
            return;
        }
        if (!/^image\/(jpeg|jpg|png|gif|webp)$/.test(file.type)) {
            alert('仅支持 jpg/png/gif/webp 格式');
            $(this).val('');
            avatarSelectedFile = null;
            $('#avatarUploadBtn').prop('disabled', true);
            return;
        }
        avatarSelectedFile = file;
        // 预览
        var reader = new FileReader();
        reader.onload = function (e) {
            $('#avatarPreview').html('<img src="' + e.target.result + '" alt="">');
        };
        reader.readAsDataURL(file);
        $('#avatarUploadBtn').prop('disabled', false);
    });

    // 上传头像（URL 来自 window.AVATAR_UPLOAD_URL）
    window.uploadAvatar = function () {
        if (!avatarSelectedFile) return;
        var formData = new FormData();
        formData.append('avatar', avatarSelectedFile);
        var $btn = $('#avatarUploadBtn');
        $btn.prop('disabled', true).text('上传中...');
        $.ajax({
            url: window.AVATAR_UPLOAD_URL,
            type: 'POST',
            data: formData,
            processData: false,
            contentType: false,
            success: function (resp) {
                // 更新导航栏头像
                $('#navAvatar').html('<img src="' + resp.avatar_url + '" alt="">');
                window.closeAvatarModal();
                // 简单提示
                var $tip = $('<div style="position:fixed;top:20px;left:50%;transform:translateX(-50%);background:#27ae60;color:white;padding:0.6rem 1.2rem;border-radius:6px;z-index:10000;box-shadow:0 4px 12px rgba(0,0,0,0.2);">头像更新成功</div>');
                $('body').append($tip);
                setTimeout(function () { $tip.fadeOut(300, function () { $(this).remove(); }); }, 2000);
            },
            error: function (xhr) {
                var msg = '头像上传失败';
                try { msg = JSON.parse(xhr.responseText).error || msg; } catch (e) {}
                alert(msg);
            },
            complete: function () {
                $btn.prop('disabled', false).text('上传');
            }
        });
    };

    // ===== 消息通知（WebSocket 推送） =====
    // 消息页 URL（点击"我的消息"整页跳转）
    var NOTI_PAGE_URL = '/notifications/list';
    var wsReconnectTimer = null;

    // 注入消息菜单项 + 未读小红点（页面加载后执行一次）
    function injectNotificationUI() {
        var $menu = $('#userDropdownMenu');
        if (!$menu.length) return; // 登录页等无下拉菜单，跳过

        // 已注入则跳过（SPA 切换页面时 navbar 不变，避免重复注入）
        if ($('#notiMenuItem').length) return;

        // 在"我的审批"后插入"我的消息"入口（直接跳转消息页，整页加载）
        var $approvalItem = $menu.find('.dropdown-item[href*="/approvals/list"]');
        var msgItem = '<a href="' + NOTI_PAGE_URL + '" class="dropdown-item" id="notiMenuItem">' +
            '&#128276; 我的消息' +
            '<span class="notification-badge" id="notiBadge" style="display:none;"></span>' +
        '</a>';
        if ($approvalItem.length) {
            $approvalItem.after(msgItem);
        } else {
            $menu.prepend(msgItem);
        }

        // 在用户名旁加小红点（与 approval-badge 并列）
        var $name = $('.user-name');
        if ($name.length && !$('#notiNameBadge').length) {
            $name.append('<span class="notification-badge" id="notiNameBadge" style="display:none;"></span>');
        }
    }

    // 更新审批红点（approvalBadge 在用户名旁，approvalBadgeNum 在下拉菜单"我的审批"项内）
    function updateApprovalBadge(count) {
        var $badge = $('#approvalBadge');
        var $num = $('#approvalBadgeNum');
        if (count > 0) {
            $badge.show().text(count);
            $num.text(count).show();
        } else {
            $badge.hide().text('');
            $num.text('').hide();
        }
    }

    // 更新通知红点（显示具体未读条数）
    function updateNotificationBadge(count) {
        if (count > 0) {
            $('#notiBadge').show().text(count);
            $('#notiNameBadge').show().text(count);
        } else {
            $('#notiBadge').hide().text('');
            $('#notiNameBadge').hide().text('');
        }
    }

    // 建立 WebSocket 连接（含断线重连）
    function connectWebSocket() {
        var proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        var wsUrl = proto + '//' + window.location.host + '/ws';
        var socket;

        try {
            socket = new WebSocket(wsUrl);
        } catch (e) {
            scheduleReconnect();
            return;
        }

        socket.onmessage = function (event) {
            try {
                var msg = JSON.parse(event.data);
                if (msg.type === 'approval_count') {
                    updateApprovalBadge(msg.count || 0);
                } else if (msg.type === 'notification_count') {
                    updateNotificationBadge(msg.count || 0);
                }
            } catch (e) { /* 忽略无法解析的消息 */ }
        };

        socket.onclose = function () {
            scheduleReconnect();
        };

        socket.onerror = function () {
            // 出错时 onclose 会触发重连，这里只主动关闭
            try { socket.close(); } catch (e) {}
        };
    }

    // 断线重连（5 秒后重试）
    function scheduleReconnect() {
        if (wsReconnectTimer) return;
        wsReconnectTimer = setTimeout(function () {
            wsReconnectTimer = null;
            connectWebSocket();
        }, 5000);
    }

    // ===== 消息中心侧边栏注入 =====
    // 在左侧导航 .nav-menu 追加"消息中心"目录（互动消息 / 审批消息）
    function injectMessageCenterNav() {
        var $menu = $('.nav-menu');
        if (!$menu.length) return;          // 无侧边栏页面（登录页等）跳过
        if ($('#msgCenterNav').length) return; // 已注入跳过

        var path = window.location.pathname;
        var activeInteraction = path.indexOf('/notifications/list') === 0 ? ' active' : '';
        var activeApproval = path.indexOf('/approvals/list') === 0 ? ' active' : '';
        // 在消息中心页时默认展开
        var expandedCls = (activeInteraction || activeApproval) ? ' expanded' : '';
        var subShowCls = (activeInteraction || activeApproval) ? ' show' : '';

        var html = '<li class="nav-item" id="msgCenterNav">' +
            '<div class="nav-header' + expandedCls + '" onclick="toggleSubMenu(this)">' +
                '<span><span class="tab-icon">&#128276;</span> 消息中心</span>' +
                '<span class="arrow">&#9654;</span>' +
            '</div>' +
            '<ul class="nav-sub-menu' + subShowCls + '">' +
                '<li><a href="/notifications/list"' + (activeInteraction ? ' class="active"' : '') + '>互动消息</a></li>' +
                '<li><a href="/approvals/list"' + (activeApproval ? ' class="active"' : '') + '>审批消息</a></li>' +
            '</ul>' +
        '</li>';
        $menu.append(html);
    }

    // 页面就绪后初始化
    $(function () {
        injectNotificationUI();
        injectMessageCenterNav();
        // 通过 WebSocket 接收审批/通知数量推送（替代轮询）
        if ($('#userDropdownMenu').length) {
            connectWebSocket();
        }
    });
})();

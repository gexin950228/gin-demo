// 文章相关的JavaScript代码

// 全局变量
var page = 1;
var limit = 2;
var total = 0;
var currentTag = null;
var user = '用户';

// 设置活跃标签
function setActiveLabel(tag){
  currentTag = tag || null;
  $('#labelList .labelBtn').removeClass('active');
  if(tag){
    $('#labelList .labelBtn').filter(function(){ return $(this).data('tag') === tag }).addClass('active');
    $('#clearFilter').show();
  } else {
    $('#clearFilter').hide();
  }
}

// 渲染文章列表
function renderArticles(articles){
  if(!articles || articles.length === 0){ 
    $('#articles').html('<p>暂无文章</p>'); 
    return;
  }
  var html = '';
  html += '<table class="articleTable">';
  html += '<thead><tr><th style="width:40%">标题</th><th style="width:20%">发布时间</th><th style="width:30%">标签</th><th style="width:10%">操作</th></tr></thead>';
  html += '<tbody>';
  articles.forEach(function(a){
    html += '<tr>';
    html += '<td class="titleCell" title="'+(a.title||'')+'">'+(a.title||'')+'</td>';
    html += '<td style="color:#666;font-size:13px">'+(new Date(a.published_at)).toLocaleString()+'</td>';
    html += '<td class="tagsCell">';
    if(a.tags && a.tags.length){
      a.tags.forEach(function(t){ html += '<button class="tagBtn" data-tag="'+t+'" title="'+t+'" aria-label="标签: '+t+'" tabindex="0" style="padding:4px 6px;font-size:12px;margin-right:6px">'+t+'</button>' });
    }
    html += '</td>';
    html += '<td class="actionsCell">';
    html += '<button class="viewBtn" data-id="'+a.id+'">查看</button>';
    if (a.author === user) {
      html += '<button class="editBtn" data-id="'+a.id+'" style="margin-left:6px">编辑</button>';
    } else {
      html += '<button class="editBtn" data-id="'+a.id+'" style="margin-left:6px;opacity:0.4;cursor:not-allowed" disabled>编辑</button>';
    }
    html += '</td>';
    html += '</tr>';
  });
  html += '</tbody></table>';
  $('#articles').html(html);

  // 标签点击和键盘事件
  $('.tagBtn').on('click', function(){
    var tag = $(this).data('tag');
    page = 1;
    setActiveLabel(tag);
    load(tag);
    $('#pageNum').text(page);
    $('#pageInfo').text('第 '+page+' 页（每页 '+limit+' 篇）' + (tag ? '，筛选标签: ' + tag : ''));
  });
  $('.tagBtn').on('keydown', function(e){ 
    if(e.key==='Enter' || e.key===' ' || e.key==='Spacebar'){
      e.preventDefault(); 
      $(this).trigger('click'); 
    }
  });

  // 查看和编辑按钮事件
  $('.viewBtn').on('click', function(e){ 
    e.preventDefault(); 
    var id = $(this).data('id'); 
    $.ajax({ 
      url: '/articles/' + id, 
      method: 'GET', 
      success: function(res){ 
        $('#modalTitle').text(res.title); 
        var meta = '作者：'+res.author+' 发布：'+(new Date(res.published_at)).toLocaleString(); 
        if(res.tags && res.tags.length){ 
          meta += ' • 标签：' + res.tags.join(', '); 
        } 
        $('#modalMeta').text(meta); 
        $('#modalBody').text(res.body); 
        $('#editFields').hide(); 
        $('#modalBodyWrap').show(); 
        $('#modalSave').hide(); 
        $('#modal').css('display','flex'); 
      }, 
      error: function(){ alert('加载失败') } 
    });
  });

  $('.editBtn').on('click', function(e){ 
    e.preventDefault(); 
    if($(this).is(':disabled')) return; 
    var id = $(this).data('id'); 
    $.ajax({ 
      url: '/articles/' + id, 
      method: 'GET', 
      success: function(articleRes){ 
        $('#modalTitle').text('编辑： '+articleRes.title); 
        $('#modalMeta').text('作者：'+articleRes.author+' 发布：'+(new Date(articleRes.published_at)).toLocaleString()); 
        $('#editTitle').val(articleRes.title); 
        $('#editBody').val(articleRes.body); 
        $('#editTagInput').val(articleRes.tags ? articleRes.tags.join(', ') : ''); 
        $('#modalBodyWrap').hide(); 
        $('#editFields').show(); 
        $('#modalSave').show(); 
        $('#modal').css('display','flex'); 
        $('#modalSave').data('id', id); 
        $.ajax({ 
          url: '/articles/labels', 
          method: 'GET', 
          success: function(labelsRes){ 
            if(labelsRes.labels && labelsRes.labels.length){ 
              var html = ''; 
              labelsRes.labels.forEach(function(l){ 
                var checked = (articleRes.tags && articleRes.tags.includes(l)) ? 'checked' : ''; 
                html += '<label style="margin-right:8px"><input type="checkbox" value="'+l+'" '+checked+'> '+l+'</label>'; 
              }); 
              $('#editExistingTags').html(html); 
            } 
          } 
        });
      }, 
      error: function(){ alert('加载失败') } 
    });
  });
}

// 渲染标签
function renderLabels(labels){
  $('#labelsLoading').remove();
  var list = $('#labelList');
  list.find('.labelBtn').remove();
  if(list.find('.labelBtn[data-tag=""]').length === 0){
    list.prepend('<button class="tagBtn labelBtn" data-tag="" aria-label="所有" tabindex="0">所有</button>');
  }
  labels.forEach(function(l){
    // 处理后端可能返回的字符串或对象
    var name = (typeof l === 'string') ? l : (l.name || l.Name || '');
    var btn = $('<button class="tagBtn labelBtn" data-tag="'+name+'" tabindex="0">'+name+'</button>');
    list.append(btn);
  });
  $('#labelList .labelBtn').on('click', function(){
    var tag = $(this).data('tag');
    page = 1;
    setActiveLabel(tag || null);
    load(tag || null);
    $('#pageNum').text(page);
    $('#pageInfo').text('第 '+page+' 页（每页 '+limit+' 篇）' + (tag ? '，筛选标签: ' + tag : ''));
  });
}

// 渲染分页
function renderPagination(){
  var totalPages = Math.ceil(total / limit);
  var paginationHtml = '';
  
  // 首页按钮
  paginationHtml += '<button class="page-btn" data-page="1" ' + (page === 1 ? 'disabled style="opacity:0.5;cursor:not-allowed"' : '') + '>首页</button>';
  
  // 页码按钮
  if(totalPages <= 5){
    // 显示所有页码
    for(var i = 1; i <= totalPages; i++){
      paginationHtml += '<button class="page-btn" data-page="' + i + '" ' + (page === i ? 'style="background:#1976d2;color:#fff"' : '') + '>' + i + '</button>';
    }
  } else {
    // 只显示当前页、首页、最后一页
    paginationHtml += '<button class="page-btn" data-page="1" ' + (page === 1 ? 'style="background:#1976d2;color:#fff"' : '') + '>1</button>';
    if(page > 2){
      paginationHtml += '<span>...</span>';
    }
    if(page > 1 && page < totalPages){
      paginationHtml += '<button class="page-btn" data-page="' + page + '" style="background:#1976d2;color:#fff">' + page + '</button>';
    }
    if(page < totalPages - 1){
      paginationHtml += '<span>...</span>';
    }
    paginationHtml += '<button class="page-btn" data-page="' + totalPages + '" ' + (page === totalPages ? 'style="background:#1976d2;color:#fff"' : '') + '>' + totalPages + '</button>';
  }
  
  // 末页按钮
  paginationHtml += '<button class="page-btn" data-page="' + totalPages + '" ' + (page === totalPages ? 'disabled style="opacity:0.5;cursor:not-allowed"' : '') + '>末页</button>';
  
  // 更新分页控件
  $('#paginationControls').html(paginationHtml);
  
  // 绑定页码点击事件
  $('.page-btn').on('click', function(){ 
    if(!$(this).is(':disabled')){
      page = parseInt($(this).data('page')); 
      load(currentTag); 
    }
  });
}

// 加载文章
function load(tag){
  currentTag = tag || null;
  setActiveLabel(currentTag);
  $('#articles').html('<p>加载中...</p>');
  var url = '/articles?page='+page+'&limit='+limit;
  if(currentTag) url += '&tag=' + encodeURIComponent(currentTag);
  console.log('加载文章', {url: url, page: page, tag: currentTag});
  $.ajax({ 
    url: url, 
    method: 'GET', 
    success: function(res){ 
      console.log('articles response', res);
      if(!res || !res.articles){
        $('#articles').html('<p>无返回文章数据</p>');
        return;
      }
      renderArticles(res.articles || []);
      total = res.total || 0;
      $('#pageNum').text(page);
      var totalPages = Math.ceil(total / limit);
      if(page <= 1) {
        $('#prev').hide();
      } else {
        $('#prev').show().prop('disabled', false);
      }
      if(page >= totalPages || (res.articles||[]).length < limit) {
        $('#next').hide();
      } else {
        $('#next').show().prop('disabled', false);
      }
      renderPagination();
    }, 
    error: function(xhr){
      var t = xhr && xhr.responseText ? xhr.responseText : '';
      $('#articles').html('<p>加载失败: '+(t||'网络错误')+'</p>');
      console.error('articles load failed', xhr);
    }
  });
}

// 初始化文章相关功能
function initArticles(username){
  user = username || '用户';
  
  // 保存按钮事件
  $('#modalSave').on('click', function(){ 
    var id = $(this).data('id'); 
    var title = $('#editTitle').val().trim(); 
    var body = $('#editBody').val().trim(); 
    if(!title || !body){ 
      alert('标题和正文不能为空'); 
      return; 
    } 
    var typed = $('#editTagInput').val().trim(); 
    var tags = []; 
    if(typed){ 
      tags = tags.concat(typed.split(',').map(function(s){ return s.trim() }).filter(Boolean)); 
    } 
    $('#editExistingTags input[type=checkbox]:checked').each(function(){ 
      var tag = $(this).val(); 
      if(!tags.includes(tag)){ 
        tags.push(tag); 
      } 
    }); 
    var token = getToken(); 
    if(!token){ 
      alert('登录已过期，请重新登录'); 
      return; 
    } 
    var articleData = {title: title, body: body, tags: tags};
    console.log('保存文章数据:', articleData); 
    $.ajax({ 
      url: '/articles/'+id, 
      method: 'PUT', 
      contentType: 'application/json', 
      data: JSON.stringify(articleData), 
      beforeSend: function(xhr){ 
        if(token) xhr.setRequestHeader('Authorization','Bearer '+token); 
      }, 
      success: function(){ 
        console.log('保存成功，重新加载文章列表'); 
        $('#modal').hide(); 
        alert('保存成功'); 
        load(currentTag); 
      }, 
      error: function(xhr){ 
        console.error('保存失败:', xhr); 
        var errorMsg = '更新失败'; 
        if(xhr.responseJSON && xhr.responseJSON.error){ 
          errorMsg = xhr.responseJSON.error; 
        } else if(xhr.responseText){ 
          errorMsg = xhr.responseText; 
        } 
        alert(errorMsg); 
      } 
    });
  });

  // 分页按钮事件
  $('#prev').on('click', function(){ if(page>1){ page--; load(currentTag); } });
  $('#next').on('click', function(){ page++; load(currentTag); });

  // 清除筛选按钮事件
  $('#clearFilter').on('click', function(){ 
    page = 1; 
    setActiveLabel(null); 
    load(null); 
    $('#pageNum').text(page); 
    $('#pageInfo').text('第 '+page+' 页'); 
  });

  // 加载标签
  $.ajax({ 
    url: '/articles/labels', 
    method: 'GET', 
    success: function(res){ 
      console.log('labels response', res); 
      renderLabels(res.labels || []); 
    }, 
    error: function(xhr){ 
      $('#labelsLoading').text('无法加载标签'); 
      console.error('labels load failed', xhr); 
    } 
  });

  // 初始加载文章
  console.log('初始加载文章列表');
  load();
}

// 导出函数
window.initArticles = initArticles;
window.loadArticles = load;

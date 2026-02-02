(function(){
  function getToken(){
    try{ var t = localStorage.getItem('token'); if(t) return t }catch(e){}
    var m = document.cookie.match(/(^|; )token=([^;]+)/);
    return m ? decodeURIComponent(m[2]) : null;
  }

  function base64UrlDecode(str){
    str = str.replace(/-/g, '+').replace(/_/g, '/');
    while(str.length % 4) str += '=';
    try{
      var bin = atob(str);
      var bytes = Array.prototype.map.call(bin, function(c){
        return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
      }).join('');
      return decodeURIComponent(bytes);
    }catch(e){ return null }
  }

  function parseJwtPayload(token){
    if(!token) return null;
    var parts = token.split('.');
    if(parts.length < 2) return null;
    var payload = base64UrlDecode(parts[1]);
    if(!payload) return null;
    try{ return JSON.parse(payload) }catch(e){ return null }
  }

  // skip redirect on these paths (login/register and API endpoints)
  var skipPrefixes = ['/users', '/static/login.html', '/static/register.html', '/favicon'];
  var path = window.location.pathname;
  for(var i=0;i<skipPrefixes.length;i++){
    if(path.indexOf(skipPrefixes[i]) === 0) return;
  }

  var token = getToken();
  if(!token){ window.location.href = '/users/to_login'; return }
  var payload = parseJwtPayload(token);
  if(!payload) { window.location.href = '/users/to_login'; return }
  var exp = payload.exp;
  if(!exp){ window.location.href = '/users/to_login'; return }
  var now = Math.floor(Date.now()/1000);
  if(exp < now){ try{ localStorage.removeItem('token') }catch(e){}; window.location.href = '/users/to_login'; return }
})();
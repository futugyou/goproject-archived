<html>
<head>
<title></title>
</head>
<body>
<form action="/login" method="post">
	用户名:<input type="text" name="username">
	密码:<input type="password" name="password">
    水果:<select name="fruit">
        <option >---</option>
        <option value="apple">apple</option>
        <option value="pear">pear</option>
        <option value="banana">banana</option>
    </select> 
    <input type="hidden" name="token" value="{{.}}">
	<input type="submit" value="登录">
</form>
</body>
</html>
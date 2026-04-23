$html = Get-Content "frontend\camera-settings.html" -Raw -Encoding UTF8
$newForms = Get-Content "replacement_form.js" -Raw -Encoding UTF8
$pattern = '(?s)        function showAddCameraModal\(\) \{.*?        async function deleteCamera\(id\) \{'
$replacement = "$newForms`r`n        async function deleteCamera(id) {"
$html = $html -replace $pattern, $replacement
[IO.File]::WriteAllText("frontend\camera-settings.html", $html, [Text.Encoding]::UTF8)

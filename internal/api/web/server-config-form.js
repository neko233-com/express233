/* The JSON editor remains the lossless source for saveServerConfig. */
let overrideFormWriting = false;
function renderServerOverrideForm() {
  if (overrideFormWriting) return;
  const host = document.getElementById('serverOverrideForm');
  const editor = document.getElementById('serverConfigJSON');
  if (!host || !editor) return;
  host.replaceChildren();
  let entry;
  try {
    entry = JSON.parse(editor.value);
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) throw new Error();
    if (entry.replacements != null && (typeof entry.replacements !== 'object' || Array.isArray(entry.replacements))) throw new Error();
  } catch (_) {
    host.textContent = '完整 JSON 格式有误，请展开高级编辑修正后继续。';
    return;
  }
  const replacements = entry.replacements || {};
  const write = () => {
    editor.value = JSON.stringify(entry, null, 2);
    overrideFormWriting = true;
    editor.dispatchEvent(new Event('input', { bubbles: true }));
    overrideFormWriting = false;
  };
  const make = (tag, className, text) => {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (text != null) el.textContent = text;
    return el;
  };
  const leafRows = [];
  const collect = (file, object, path = []) => {
    for (const [key, value] of Object.entries(object)) {
      const fullPath = [...path, key];
      if (value && typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length) collect(file, value, fullPath);
      else leafRows.push({ file, object, key, path: fullPath, value });
    }
  };
  for (const [file, values] of Object.entries(replacements)) {
    if (values && typeof values === 'object' && !Array.isArray(values)) collect(file, values);
  }
  const note = make('p', 'hint', '按文件修改参数。文本、数字和开关保留原类型；保存后可预览最终配置。');
  host.append(note);
  if (!leafRows.length) host.append(make('p', 'agent-empty', '还没有分服参数。在下方添加端口、区服编号等需要覆盖的值。'));
  let lastFile = '';
  for (const row of leafRows) {
    if (lastFile !== row.file) host.append(make('h3', 'override-file-name', row.file));
    lastFile = row.file;
    const label = make('label', 'override-field');
    label.append(make('span', '', row.path.join('.')));
    let input;
    const scalar = row.value == null || ['string', 'number', 'boolean'].includes(typeof row.value);
    if (typeof row.value === 'boolean') {
      input = make('select', 'input');
      for (const value of ['true', 'false']) {
        const option = make('option', '', value === 'true' ? '开启 (true)' : '关闭 (false)');
        option.value = value;
        input.append(option);
      }
    } else input = make('input', 'input');
    input.value = typeof row.value === 'object' ? JSON.stringify(row.value) : String(row.value);
    input.setAttribute('aria-label', `${row.file} / ${row.path.join('.')}`);
    if (!scalar || row.value === null) { input.readOnly = true; input.title = '此类型请在完整 JSON 中编辑'; }
    else input.addEventListener('input', () => {
      let value = input.value;
      if (typeof row.value === 'number') {
        if (!value.trim() || !Number.isFinite(Number(value))) { input.setCustomValidity('请输入有效数字'); input.reportValidity(); return; }
        value = Number(value);
      } else if (typeof row.value === 'boolean') value = value === 'true';
      input.setCustomValidity('');
      row.object[row.key] = value;
      write();
    });
    label.append(input);
    host.append(label);
  }
  const add = make('form', 'override-add');
  const file = make('input', 'input');
  file.placeholder = '文件名，如 application.yaml';
  file.setAttribute('aria-label', '覆盖文件名');
  file.required = true;
  const key = make('input', 'input');
  key.placeholder = '参数名，如 server.port';
  key.setAttribute('aria-label', '覆盖参数名');
  key.required = true;
  const value = make('input', 'input');
  value.placeholder = '参数值';
  value.setAttribute('aria-label', '覆盖参数值');
  const type = make('select', 'input');
  type.setAttribute('aria-label', '参数值类型');
  for (const [id, name] of [['string', '文本'], ['number', '数字'], ['boolean', '开关 true / false']]) {
    const option = make('option', '', name); option.value = id; type.append(option);
  }
  const button = make('button', 'btn btn-secondary btn-sm', '添加参数');
  button.type = 'submit';
  const error = make('p', 'warn'); error.setAttribute('role', 'status');
  add.append(file, key, value, type, button, error);
  add.addEventListener('submit', (event) => {
    event.preventDefault();
    const fileName = file.value.trim();
    const keyName = key.value.trim();
    if (!/^[^/\\]+\.(?:json|ya?ml|properties)$/i.test(fileName) || !keyName) { error.textContent = '请填写配置文件名（不含目录）和参数名。'; return; }
    let nextValue = value.value;
    if (type.value === 'number') {
      if (!nextValue.trim() || !Number.isFinite(Number(nextValue))) { error.textContent = '数字类型需要有效数字。'; return; }
      nextValue = Number(nextValue);
    } else if (type.value === 'boolean') {
      if (!['true', 'false'].includes(nextValue.trim())) { error.textContent = '开关类型请输入 true 或 false。'; return; }
      nextValue = nextValue.trim() === 'true';
    }
    const target = Object.hasOwn(replacements, fileName) ? replacements[fileName] : {};
    if (!target || typeof target !== 'object' || Array.isArray(target)) { error.textContent = '请先在完整 JSON 中修正该文件的参数结构。'; return; }
    const paths = leafRows.filter(row => row.file === fileName).map(row => row.path.join('.'));
    if (paths.some(path => path === keyName || path.startsWith(keyName + '.') || keyName.startsWith(path + '.'))) { error.textContent = '此参数已存在或与现有结构冲突，请直接编辑原参数。'; return; }
    // Define own properties so names such as __proto__ cannot mutate prototypes.
    Object.defineProperty(target, keyName, { value: nextValue, enumerable: true, writable: true, configurable: true });
    Object.defineProperty(replacements, fileName, { value: target, enumerable: true, writable: true, configurable: true });
    entry.replacements = replacements;
    write();
    renderServerOverrideForm();
  });
  host.append(add);
}

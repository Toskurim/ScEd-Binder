// Estrae i modelli di periferica da SC Keymap SENZA eseguirne il codice:
// legge i sorgenti come testo, li analizza con il parser di TypeScript e valuta solo espressioni di dati.
import ts from 'typescript';
import { readFileSync, writeFileSync } from 'fs';
const DIR = '/tmp/dl/sc-mapper/src/lib/';
function load(file) {
  const sf = ts.createSourceFile(file, readFileSync(DIR + file, 'utf8'), ts.ScriptTarget.Latest, true);
  const consts = new Map(), calls = [];
  for (const st of sf.statements) {
    if (ts.isVariableStatement(st)) for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name) && d.initializer) consts.set(d.name.text, d.initializer);
    if (ts.isExpressionStatement(st) && ts.isCallExpression(st.expression) && ts.isIdentifier(st.expression.expression)) calls.push(st.expression);
  }
  return { consts, calls };
}
const K = ts.SyntaxKind;
const native = {
  b: (...n) => n.map((i) => `button${i}`),
  hat: (n) => ['up', 'right', 'down', 'left'].map((d) => `hat${n}_${d}`),
  u: (n) => Array.from({ length: n }, () => ''),
  range: (a, z) => Array.from({ length: z - a + 1 }, (_, i) => a + i),
  c: (id, kind, inputs, label, group, more = {}) => ({ id, kind, inputs, label, group, ...more }),
};
function makeEval(consts) {
  const cache = new Map();
  function ev(n, env) {
    switch (n.kind) {
      case K.StringLiteral: case K.NoSubstitutionTemplateLiteral: return n.text;
      case K.NumericLiteral: return Number(n.text);
      case K.TrueKeyword: return true; case K.FalseKeyword: return false; case K.NullKeyword: return null;
      case K.ParenthesizedExpression: case K.AsExpression: case K.NonNullExpression: case K.SatisfiesExpression: return ev(n.expression, env);
      case K.PrefixUnaryExpression: { const v = ev(n.operand, env); if (n.operator === K.MinusToken) return -v; if (n.operator === K.ExclamationToken) return !v; throw new Error('unary'); }
      case K.TemplateExpression: return n.head.text + n.templateSpans.map((s) => String(ev(s.expression, env)) + s.literal.text).join('');
      case K.ArrayLiteralExpression: { const out = []; for (const e of n.elements) { if (ts.isSpreadElement(e)) out.push(...ev(e.expression, env)); else out.push(ev(e, env)); } return out; }
      case K.ObjectLiteralExpression: { const o = {}; for (const p of n.properties) {
        if (ts.isSpreadAssignment(p)) Object.assign(o, ev(p.expression, env));
        else if (ts.isShorthandPropertyAssignment(p)) o[p.name.text] = ev(p.name, env);
        else if (ts.isPropertyAssignment(p)) o[ts.isComputedPropertyName(p.name) ? ev(p.name.expression, env) : p.name.text] = ev(p.initializer, env);
        else throw new Error('objprop'); } return o; }
      case K.Identifier: {
        if (n.text === 'undefined') return undefined;
        if (env && n.text in env) return env[n.text];
        if (n.text in native) return native[n.text];
        if (cache.has(n.text)) return cache.get(n.text);
        if (consts.has(n.text)) { const v = ev(consts.get(n.text), null); cache.set(n.text, v); return v; }
        throw new Error('id ' + n.text);
      }
      case K.BinaryExpression: { const o = n.operatorToken.kind;
        if (o === K.QuestionQuestionToken) { const l = ev(n.left, env); return l ?? ev(n.right, env); }
        const l = ev(n.left, env), r = ev(n.right, env);
        if (o === K.PlusToken) return l + r; if (o === K.MinusToken) return l - r; if (o === K.AsteriskToken) return l * r; if (o === K.SlashToken) return l / r;
        throw new Error('binop'); }
      case K.ConditionalExpression: return ev(n.condition, env) ? ev(n.whenTrue, env) : ev(n.whenFalse, env);
      case K.ArrowFunction: {
        return (...args) => { const e2 = { ...(env || {}) }; n.parameters.forEach((p, i) => { if (!ts.isIdentifier(p.name)) throw new Error('param'); e2[p.name.text] = args[i] === undefined && p.initializer ? ev(p.initializer, e2) : args[i]; });
          if (ts.isBlock(n.body)) { for (const s of n.body.statements) { if (ts.isReturnStatement(s)) return ev(s.expression, e2); if (ts.isVariableStatement(s)) { for (const d of s.declarationList.declarations) e2[d.name.text] = ev(d.initializer, e2); continue; } throw new Error('stmt'); } return undefined; }
          return ev(n.body, e2); };
      }
      case K.PropertyAccessExpression: { const o = ev(n.expression, env); return o[n.name.text]; }
      case K.ElementAccessExpression: return ev(n.expression, env)[ev(n.argumentExpression, env)];
      case K.CallExpression: {
        const args = []; for (const a of n.arguments) { if (ts.isSpreadElement(a)) args.push(...ev(a.expression, env)); else args.push(ev(a, env)); }
        if (ts.isPropertyAccessExpression(n.expression)) {
          const o = ev(n.expression.expression, env), m = n.expression.name.text;
          if ((Array.isArray(o) && ['map', 'filter', 'concat', 'slice', 'flat', 'flatMap', 'join', 'includes'].includes(m)) || (typeof o === 'string' && ['replace', 'slice', 'trim'].includes(m))) return o[m](...args);
          if (ts.isIdentifier(n.expression.expression) && n.expression.expression.text === 'Math' ) return Math[m](...args);
          if (ts.isIdentifier(n.expression.expression) && n.expression.expression.text === 'Array' && m === 'from') return Array.from(...args);
          throw new Error('method ' + m);
        }
        const f = ev(n.expression, env); if (typeof f !== 'function') throw new Error('call'); return f(...args);
      }
      default: throw new Error('kind ' + K[n.kind]);
    }
  }
  return ev;
}
const T = load('deviceTemplates.ts'), L = load('devicePhotoLayouts.ts'), Z = load('devicePhotoSizes.ts');
const sizes = makeEval(Z.consts)(Z.consts.get('DEVICE_PHOTO_SIZES'), null);
const layouts = makeEval(L.consts)(L.consts.get('DEVICE_PHOTO_LAYOUTS'), null);
native.DEVICE_PHOTO_SIZES = sizes; native.PHOTO_LABEL_GUTTER = 0.3;
const ev = makeEval(T.consts);
const out = [], problems = [];
for (const call of T.calls) {
  const fn = call.expression.text;
  if (!['tpl', 'photoTpl', 'photoTplExact', 'photoTplExactViews', 'pedalTpl'].includes(fn)) continue;
  let args; try { args = call.arguments.map((a) => ev(a, null)); } catch (e) { problems.push(fn + ' @' + call.pos + ': ' + e.message); continue; }
  out.push({ fn, args });
}
writeFileSync('/tmp/ext/raw.json', JSON.stringify({ calls: out, layouts, sizes, problems }));
console.log('calls', out.length, 'problems', problems);

// Regression for the screenshot's 154px cards and overflowing single-line labels.
// Synthetic geometry tests; not a browser rendering pass.
const assert = require('node:assert/strict');
const {containsText, checkTextFit} = require('../../kernel/.agents/skills/explain-visually/scripts/check_text_fit.cjs');
const oldCard = {left:42, top:112, right:196, bottom:228};
assert.equal(containsText(oldCard,{left:59,top:184,right:260,bottom:205}),false);
const wideCard = {left:36,top:100,right:220,bottom:244};
assert.equal(containsText(wideCard,{left:50,top:184,right:180,bottom:223}),true);
assert.equal(containsText(wideCard,{left:40,top:184,right:180,bottom:223}),false,'padding required');
assert.equal(containsText(wideCard,{left:50,top:184,right:180,bottom:240}),false,'vertical overflow');
(async()=>{
 const empty=await checkTextFit({querySelectorAll:()=>[]});
 assert.equal(empty.text_fit_pass,false,'missing measurements cannot pass');
 let fontsReady=false;
 const matrix={a:1,b:0,c:0,d:1,e:0,f:0};
 const box={id:'card',getClientRects:()=>[1],hasAttribute:()=>true,getAttribute:()=>null,getScreenCTM:()=>matrix,getBBox:()=>({x:42,y:112,width:154,height:116})};
 const label={textContent:'What needs an answer?',getClientRects:()=>[1],getAttribute:()=> 'card',getScreenCTM:()=>matrix,getBBox:()=>({x:59,y:184,width:201,height:21})};
 const doc={fonts:{ready:Promise.resolve().then(()=>{fontsReady=true;})},querySelectorAll:q=>q==='[data-text-box]'?[box]:[label],getElementById:()=>box};
 assert.equal((await checkTextFit(doc)).text_fit_pass,false);
 assert.equal(fontsReady,true);
 box.getBBox=()=>({x:36,y:100,width:184,height:144});
 label.getBBox=()=>({x:50,y:184,width:130,height:39});
 assert.equal((await checkTextFit(doc)).text_fit_pass,true);
 console.log('7 text-fit regression cases passed; supplied geometry, browser unverified');
})();

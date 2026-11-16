import re,json,shutil
import xml.etree.ElementTree as ET
D='/tmp/dl/'
def ini(p):
    d={}
    for line in open(p,encoding='utf-8-sig',errors='replace'):
        if '=' in line:
            k,v=line.rstrip('\r\n').split('=',1)
            d[k.split(',')[0].strip().lower()]=v.replace('\\n',' ').strip()
    return d
en=ini(D+'loc/data/Localization/english/global.ini')
it=ini(D+'loc/data/Localization/italian_(italy)/global.ini')
src=D+'sc-mapper/data/game/defaultProfile.xml'
x=open(src,encoding='utf-8').read()
keys=sorted(set(k.lower() for k in re.findall(r'UI(?:Label|Description|Category)="@([^"]+)"',x)))
emo={'agree':"D'accordo",'angry':'Arrabbiato','atease':'Riposo','attention':'Attenti','blah':'Bla bla','bored':'Annoiato','bow':'Inchino','burp':'Rutto','cheer':'Esulta','chicken':'Gallina','clap':'Applauso','come':'Vieni qui','cry':'Piangi','dance':'Balla','disagree':'Non sono d\'accordo','failure':'Fallimento','flex':'Mostra i muscoli','flirt':'Flirta','forward':'Avanti','gasp':'Sussulto','gloat':'Gongola','greet':'Saluta','laugh':'Ridi','launch':'Conferma lancio','left':'A sinistra','no':'No','point':'Indica','right':'A destra','rude':'Gestaccio','salute':'Saluto militare','sit':'Siediti','sleep':'Dormi','smell':'Annusa','stop':'Alt','taunt':'Provoca','threaten':'Minaccia','wait':'Aspetta','wave':'Saluta con la mano','whistle':'Fischia','yes':'Sì'}
ov={('ui_ciemote'+k):'Emote - '+v for k,v in emo.items()}
ov.update({
'ui_cc_drivemodes':'MODALITÀ DI GUIDA','ui_cceva':'E.V.A.','ui_ccevazgt':'E.V.A. - GRAVITÀ ZERO','ui_ccflightmodes':'MODALITÀ DI VOLO',
'ui_ccorientationcontrol':'CONTROLLO ORIENTAMENTO','ui_ccseatgeneral':'SEDILI - GENERALE','ui_ccturrets':'TORRETTE','ui_ccvehicle':'VEICOLI TERRESTRI',
'ui_cifpsincreaseminingthrottle':'Aumenta potenza laser minerario','ui_ciafterburner':'Postbruciatore (Boost)','ui_ciboost':'Boost',
'ui_ciautoland':'Atterraggio automatico','ui_cieject':'Espulsione (Eject)','ui_cievaboost':'Boost','ui_cifpsmovemodesprint':'Scatto (Sprint)',
'ui_cibrake':'Freno spaziale','ui_cgadvancamcontrols':'Telecamera - Controlli avanzati','ui_cifpsaimdownsight':'Mira (ADS)',
'ui_cifpsmoveleft':'Movimento - sinistra','ui_cifpsmoveright':'Movimento - destra','ui_cifpsweaponchangefiremode':'Cambia modalità di fuoco',
'ui_v_dock_toggle_view':'Attiva/disattiva telecamera di attracco','ui_cisalvagenudgeleft_right':"Sposta verso sinistra l'utensile di salvataggio destro",
'ui_cispectatorrotatepitch':'Telecamera spettatore - Rotazione pitch','ui_cispectatorrotateyaw':'Telecamera spettatore - Rotazione yaw',
'ui_cispectatortargetlock':'Telecamera spettatore - Blocca bersaglio','ui_cispectatortargetnext':'Telecamera spettatore - Bersaglio successivo','ui_cispectatortargetprev':'Telecamera spettatore - Bersaglio precedente',
'ui_cispectatormodenext':'Telecamera spettatore - Modalità successiva','ui_cispectatormodeprev':'Telecamera spettatore - Modalità precedente',
'ui_cipitchup':'Pitch su','ui_cirotateviewup':'Guarda su','ui_cirotateviewleft':'Guarda a sinistra','ui_cistrafeup':'Strafe su','ui_cistrafeupdown':'Strafe su/giù (asse)',
'ui_ciinteractionpersonal':'Personal Inner Thought (PIT)','ui_ciuichatfocus':'Attiva finestra di chat','ui_storeall':'Deposita tutte le merci',
'ui_grenade_quickselectradialmenu':'Menu radiale - Selezione oggetti da lancio','ui_weapon_quickselectradialmenu':'Menu radiale - Selezione arma','ui_consumable_quickselectradialmenu':'Menu radiale - Selezione consumabili',
'ui_shipsystems_pitwheelcategory':'Sistemi della nave - Categoria PIT','ui_v_weapon_pip_fade_off':'Dissolvenza PIP disattivata','ui_v_weapon_pip_fade_on':'Dissolvenza PIP attivata',
'ui_cimodifyport':'Interazione modifica componente','ui_citargetingpinselected':'Fissa (pin) il bersaglio selezionato','ui_citargetingpinselectedhold':'Fissa (pin) il bersaglio selezionato (tieni premuto)',
'ui_citargetingunpintarget':'Rimuovi il pin dal bersaglio selezionato','ui_citargetingunpintargethold':'Rimuovi il pin dal bersaglio selezionato (tieni premuto)',
'ui_citargetingremoveallpinned':'Rimuovi tutti i bersagli fissati (pin)',
'ui_cifpsstanceprone':'Sdraiati (prono)','ui_cgspaceflightcockpit':'Veicoli - Cabina di pilotaggio','ui_cgseatgeneral':'Veicoli - Sedili e modalità operatore',
'ui_cgspaceflighttargetingadvanced':'Veicoli - Ciclo bersagli','ui_cgspaceflightview':'Veicoli - Visuale','ui_cifpsreload':'Ricarica','ui_cifpsmovemodewalk':'Cammina',
'ui_cifpsholdbreath':'Trattieni il respiro (ADS)','ui_cipowerthrottledown':'Diminuisci potenza','ui_cipowerthrottleup':'Aumenta potenza','ui_cipowerthrottlemax':'Potenza al massimo','ui_cipowerthrottlemin':'Potenza al minimo',
'ui_cibombs_hudrangedecrease':'Bombe - Diminuisci portata HUD','ui_cibombs_hudrangeincrease':'Bombe - Aumenta portata HUD','ui_cibombs_hudrangereset':'Bombe - Ripristina portata HUD',
'ui_cibombs_toggledesiredimpactpoint':'Bombe - Punto di impatto desiderato (tocca)','ui_cibombs_toggledesiredimpactpointhold':'Bombe - Punto di impatto desiderato (tieni premuto)',
'ui_ci_countermeasures_decoy_increment':'Decoy: aumenta dimensione burst (tocca)','ui_ci_countermeasures_decoy_decrement':'Decoy: riduci dimensione burst (tocca)',
'ui_ci_countermeasures_launch_decoy':'Decoy: lancia burst (tocca), imposta e lancia burst (tieni premuto)','ui_cimissiles_decreasemax':'Diminuisci il numero di missili armati',
'ui_v_weapon_ui_scale_off':'Ingrandimento interfaccia di tiro - Off','ui_v_weapon_ui_scale_on':'Ingrandimento interfaccia di tiro - On','ui_v_weapon_ui_scale_toggle':'Ingrandimento interfaccia di tiro - Attiva/disattiva',
'ui_cifpsmeleeattackheavyright':'Mischia - attacco pesante a destra (tieni premuto)','ui_cirespawn':'Rinasci (Respawn)','ui_ciscanningactivate':'Attiva scansione',
'ui_ciscanningradarping':'Lancia ping (tieni premuto e rilascia)','ui_cgspaceflighttargeting':'Veicoli - Puntamento bersagli','ui_cgspaceflightdefensive':'Veicoli - Scudi e contromisure',
'ui_cguiinvite':'Sociale - Inviti','ui_ciinteractionfocus':'Focus','ui_cicockpitflightready':'Pronto al volo / Sistemi pronti','ui_ciexit':'Esci dal sedile',
'ui_cideploylandingsystem':'Carrello di atterraggio (estendi)','ui_ciretractlandingsystem':'Carrello di atterraggio (ritrai)','ui_citogglelandingsystem':'Carrello di atterraggio (estendi/ritrai)',
'ui_v_master_mode_cycle':'Cambia Master Mode (pressione breve)','ui_v_master_mode_cycle_long':'Cambia Master Mode (pressione lunga)','ui_v_master_mode_set_nav':'Imposta Master Mode su NAV','ui_v_master_mode_set_scm':'Imposta Master Mode su SCM',
'ui_cijumpdrivejumprequest':'Jump Drive - Richiedi salto','ui_cilookbehind':'Guarda dietro','ui_cihailtarget':'Chiama il bersaglio (Hail)','ui_ciselfdestruct':'Autodistruzione',
'ui_cijettisonvolatilecargo':'Sgancia il carico','ui_cipowerseton':'Accendi alimentazione','ui_cipowersetoff':'Spegni alimentazione',
'ui_cipowershieldsseton':'Accendi scudi','ui_cipowershieldssetoff':'Spegni scudi','ui_cipowerthrusterseton':'Accendi propulsori','ui_cipowerthrustersetoff':'Spegni propulsori','ui_cipowerweaponsseton':'Accendi armi','ui_cipowerweaponssetoff':'Spegni armi',
'ui_cicoolerthrottledown':'Diminuisci raffreddamento','ui_cicoolerthrottleup':'Aumenta raffreddamento',
'ui_v_strafe_back':'Manetta - Diminuisci','ui_v_strafe_forward':'Manetta - Aumenta','ui_v_strafe_longitudinal':'Manetta - Avanti/indietro (asse)','ui_cistrafelongitudinalinvert':'Manetta - Inverti avanti/indietro',
'ui_v_ifcs_throttle_set_normal':'Manetta - Modalità crociera: disattiva','ui_v_ifcs_throttle_set_sticky':'Manetta - Modalità crociera: attiva','ui_v_ifcs_throttle_swap_mode':'Manetta - Modalità crociera: attiva/disattiva',
})
for k in list(ov):
    if 'strafe_trim' in k: pass
for k,v in list(it.items()):
    if k.startswith('ui_v_strafe_trim') and not k.endswith('desc') and v.startswith('Trim'):
        ov.setdefault(k,'Manetta - '+v)
fix=[('Fotocamera','Telecamera'),('fotocamera','telecamera'),(' sù',' su'),('modalitÀ','modalità'),('Muovimento','Movimento'),('Moviemnto','Movimento'),('utesili','utensili'),('orizontale','orizzontale'),('trought','Thought')]
EN={};IT={}
for k in keys:
    e=en.get(k,'').strip()
    i=ov.get(k) or it.get(k,'').strip()
    for a,b in fix: i=i.replace(a,b)
    if e: EN[k]=e
    if i: IT[k]=i
    elif k in ov: IT[k]=ov[k]
json.dump(EN,open('scbind/data/labels_en.json','w'),ensure_ascii=False,indent=0,sort_keys=True)
json.dump(IT,open('scbind/data/labels_it.json','w'),ensure_ascii=False,indent=0,sort_keys=True)
shutil.copy(src,'scbind/data/defaultProfile.xml')
print(len(keys),len(EN),len(IT))
root=ET.fromstring(x)
miss=set()
for am in root.iter('actionmap'):
    for a in am.findall('action'):
        l=a.get('UILabel')
        if l and l.startswith('@') and l[1:].lower() not in IT: miss.add(l)
print('labels without IT:',miss)
print(open(D+'sc-mapper/data/game/build_manifest.json').read())

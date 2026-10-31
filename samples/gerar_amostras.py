#!/usr/bin/env python3
"""Gera XMLs fiscais de amostra (dados fictícios) para testes do Vorticis Monitor.

Uso: python3 samples/gerar_amostras.py [diretorio_saida]
"""
import os
import sys
from datetime import datetime, timedelta, timezone

OUT = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.abspath(__file__))
TZ = timezone(timedelta(hours=-3))
AGORA = datetime.now(TZ).replace(microsecond=0)


def dv_cnpj(base12):
    def dv(b, pesos):
        s = sum((ord(c) - 48) * p for c, p in zip(b, pesos))
        r = s % 11
        return 0 if r < 2 else 11 - r
    d1 = dv(base12, [5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2])
    d2 = dv(base12 + str(d1), [6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2])
    return base12 + f"{d1}{d2}"


def dv_chave(c43):
    s, p = 0, 2
    for ch in reversed(c43):
        s += (ord(ch) - 48) * p
        p = 2 if p == 9 else p + 1
    r = s % 11
    return c43 + str(0 if r < 2 else 11 - r)


def ie_sp(base8, meio2):
    p1 = [1, 3, 4, 5, 6, 7, 8, 10]
    d9 = (sum(int(c) * p for c, p in zip(base8, p1)) % 11) % 10
    s = base8 + str(d9) + meio2
    p2 = [3, 2, 10, 9, 8, 7, 6, 5, 4, 3, 2]
    d12 = (sum(int(c) * p for c, p in zip(s, p2)) % 11) % 10
    return s + str(d12)


def ie_rs(base9):
    p = [2, 9, 8, 7, 6, 5, 4, 3, 2]
    r = sum(int(c) * w for c, w in zip(base9, p)) % 11
    return base9 + str(0 if r < 2 else 11 - r)


def ie_pr(base8):
    def m(b, p):
        r = sum(int(c) * w for c, w in zip(b, p)) % 11
        return 0 if r < 2 else 11 - r
    d1 = m(base8, [3, 2, 7, 6, 5, 4, 3, 2])
    d2 = m(base8 + str(d1), [4, 3, 2, 7, 6, 5, 4, 3, 2])
    return base8 + f"{d1}{d2}"


EMPRESA = dv_cnpj("112223330001")          # 11222333000181 (Vorticis Demo, SP, Lucro Real)
EMPRESA_IE = ie_sp("11004249", "01")
FORNEC_RS = dv_cnpj("876543210001")
FORNEC_RS_IE = ie_rs("096123456")
CLIENTE_PR = dv_cnpj("456789120001")
CLIENTE_PR_IE = ie_pr("90123456")
TRANSP = dv_cnpj("334455660001")
PRESTADOR = dv_cnpj("223344550001")
CPF_CLIENTE = "52998224725"
UF_COD = {"SP": "35", "RS": "43", "PR": "41", "BA": "29"}


def chave(uf, dh, cnpj, mod, serie, nnf, tpemis, cnf):
    return dv_chave(f"{UF_COD[uf]}{dh:%y%m}{cnpj}{mod}{serie:03d}{nnf:09d}{tpemis}{cnf:08d}")


def r2(v):
    return f"{v:.2f}"


def item_xml(n, prod, icms, pis_cst="01", pis_aliq=1.65, cof_aliq=7.6, ufdest=None, ibscbs=True, ipi=None, extra_prod=""):
    v = prod["vProd"]
    pis = round(v * pis_aliq / 100, 2)
    cof = round(v * cof_aliq / 100, 2)
    pis_grp = (f"<PISAliq><CST>{pis_cst}</CST><vBC>{r2(v)}</vBC><pPIS>{pis_aliq:.4f}</pPIS><vPIS>{r2(pis)}</vPIS></PISAliq>"
               if pis_cst in ("01", "02") else f"<PISOutr><CST>{pis_cst}</CST><vBC>0.00</vBC><pPIS>0.0000</pPIS><vPIS>0.00</vPIS></PISOutr>")
    cof_grp = (f"<COFINSAliq><CST>{pis_cst}</CST><vBC>{r2(v)}</vBC><pCOFINS>{cof_aliq:.4f}</pCOFINS><vCOFINS>{r2(cof)}</vCOFINS></COFINSAliq>"
               if pis_cst in ("01", "02") else f"<COFINSOutr><CST>{pis_cst}</CST><vBC>0.00</vBC><pCOFINS>0.0000</pCOFINS><vCOFINS>0.00</vCOFINS></COFINSOutr>")
    ibs = ""
    if ibscbs:
        ibs = (f"<IBSCBS><CST>000</CST><cClassTrib>000001</cClassTrib><gIBSCBS><vBC>{r2(v)}</vBC>"
               f"<gIBSUF><pIBSUF>0.1000</pIBSUF><vIBSUF>{r2(v*0.001)}</vIBSUF></gIBSUF>"
               f"<gIBSMun><pIBSMun>0.0000</pIBSMun><vIBSMun>0.00</vIBSMun></gIBSMun><vIBS>{r2(v*0.001)}</vIBS>"
               f"<gCBS><pCBS>0.9000</pCBS><vCBS>{r2(v*0.009)}</vCBS></gCBS></gIBSCBS></IBSCBS>")
    ipi_xml = ""
    if ipi:
        ipi_xml = f"<IPI><cEnq>999</cEnq><IPITrib><CST>{ipi[0]}</CST><vBC>{r2(v)}</vBC><pIPI>{ipi[1]:.4f}</pIPI><vIPI>{r2(v*ipi[1]/100)}</vIPI></IPITrib></IPI>"
    return (f'<det nItem="{n}"><prod><cProd>{prod["cProd"]}</cProd><cEAN>SEM GTIN</cEAN><xProd>{prod["xProd"]}</xProd>'
            f'<NCM>{prod["NCM"]}</NCM>{"<CEST>"+prod["CEST"]+"</CEST>" if prod.get("CEST") else ""}<CFOP>{prod["CFOP"]}</CFOP>'
            f'<uCom>UN</uCom><qCom>{prod["q"]:.4f}</qCom><vUnCom>{prod["vUn"]:.4f}</vUnCom><vProd>{r2(v)}</vProd>'
            f'<cEANTrib>SEM GTIN</cEANTrib><uTrib>UN</uTrib><qTrib>{prod["q"]:.4f}</qTrib><vUnTrib>{prod["vUn"]:.4f}</vUnTrib><indTot>1</indTot>{extra_prod}</prod>'
            f'<imposto><ICMS>{icms}</ICMS>{ipi_xml}<PIS>{pis_grp}</PIS><COFINS>{cof_grp}</COFINS>{ufdest or ""}{ibs}</imposto></det>'), pis, cof


def nfe(nome, uf_emit, emit, dest, ide_extra, itens, tot, tpemis="1", tpnf="1", iddest="2", prot=True, nnf=1, dh=None, fin="1", indfinal="0", nfref="", mod="55"):
    dh = dh or AGORA - timedelta(hours=2)
    ch = chave(uf_emit, dh, emit["CNPJ"], mod, 1, nnf, tpemis, 10000000 + nnf)
    dets = "".join(i[0] for i in itens)
    ide = (f'<ide><cUF>{UF_COD[uf_emit]}</cUF><cNF>{10000000+nnf:08d}</cNF><natOp>{ide_extra}</natOp><mod>{mod}</mod><serie>1</serie>'
           f'<nNF>{nnf}</nNF><dhEmi>{dh.isoformat()}</dhEmi><tpNF>{tpnf}</tpNF><idDest>{iddest}</idDest><cMunFG>{emit["cMun"]}</cMunFG>'
           f'<tpImp>1</tpImp><tpEmis>{tpemis}</tpEmis><cDV>{ch[-1]}</cDV><tpAmb>2</tpAmb><finNFe>{fin}</finNFe><indFinal>{indfinal}</indFinal>'
           f'<indPres>1</indPres><procEmi>0</procEmi><verProc>ERP 1.0</verProc>{tot.pop("_cont", "")}{nfref}</ide>')
    emit_x = (f'<emit><CNPJ>{emit["CNPJ"]}</CNPJ><xNome>{emit["xNome"]}</xNome><enderEmit><xLgr>Rua A</xLgr><nro>1</nro><xBairro>Centro</xBairro>'
              f'<cMun>{emit["cMun"]}</cMun><xMun>{emit["xMun"]}</xMun><UF>{emit["UF"]}</UF><CEP>01000000</CEP></enderEmit><IE>{emit["IE"]}</IE><CRT>{emit["CRT"]}</CRT></emit>')
    did = f'<CNPJ>{dest["CNPJ"]}</CNPJ>' if "CNPJ" in dest else f'<CPF>{dest["CPF"]}</CPF>'
    dest_x = (f'<dest>{did}<xNome>{dest["xNome"]}</xNome><enderDest><xLgr>Rua B</xLgr><nro>2</nro><xBairro>Centro</xBairro><cMun>{dest["cMun"]}</cMun>'
              f'<xMun>{dest["xMun"]}</xMun><UF>{dest["UF"]}</UF><CEP>80000000</CEP></enderDest><indIEDest>{dest["indIEDest"]}</indIEDest>'
              f'{"<IE>"+dest["IE"]+"</IE>" if dest.get("IE") else ""}</dest>')
    totx = "<total><ICMSTot>" + "".join(f"<{k}>{r2(v)}</{k}>" for k, v in tot.items()) + "</ICMSTot></total>"
    inf = f'<infNFe versao="4.00" Id="NFe{ch}">{ide}{emit_x}{dest_x}{dets}{totx}<transp><modFrete>0</modFrete></transp><pag><detPag><tPag>15</tPag><vPag>{r2(tot["vNF"])}</vPag></detPag></pag></infNFe>'
    corpo = f'<NFe xmlns="http://www.portalfiscal.inf.br/nfe">{inf}</NFe>'
    if prot:
        corpo = (f'<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00">{corpo}<protNFe versao="4.00"><infProt><tpAmb>2</tpAmb>'
                 f'<verAplic>SP_NFE_PL009</verAplic><chNFe>{ch}</chNFe><dhRecbto>{(dh+timedelta(seconds=3)).isoformat()}</dhRecbto>'
                 f'<nProt>1352600000{nnf:05d}</nProt><digVal>x</digVal><cStat>100</cStat><xMotivo>Autorizado o uso da NF-e</xMotivo></infProt></protNFe></nfeProc>')
    with open(os.path.join(OUT, nome), "w", encoding="utf-8") as f:
        f.write('<?xml version="1.0" encoding="UTF-8"?>' + corpo)
    return ch


def tot_base(vprod, vicms=0, vbc=0, vpis=0, vcof=0, vbcst=0, vst=0, vipi=0, difal=0, fcp=0):
    return {"vBC": vbc, "vICMS": vicms, "vICMSDeson": 0, "vFCPUFDest": fcp, "vICMSUFDest": difal, "vFCP": 0, "vBCST": vbcst, "vST": vst,
            "vFCPST": 0, "vFCPSTRet": 0, "vProd": vprod, "vFrete": 0, "vSeg": 0, "vDesc": 0, "vII": 0, "vIPI": vipi, "vIPIDevol": 0,
            "vPIS": vpis, "vCOFINS": vcof, "vOutro": 0, "vNF": vprod + vst + vipi}


emp = {"CNPJ": EMPRESA, "xNome": "VORTICIS DEMO COMERCIO LTDA", "cMun": "3550308", "xMun": "Sao Paulo", "UF": "SP", "IE": EMPRESA_IE, "CRT": "3"}

# 1) Venda interestadual a consumidor final (DIFAL correto, base dupla BA 20,5%)
v = 1000.0
bcdest = round((v - v * 0.07) / (1 - 0.205), 2)
difal = round(bcdest * 0.205 - v * 0.07, 2)
uf = (f"<ICMSUFDest><vBCUFDest>{r2(bcdest)}</vBCUFDest><vBCFCPUFDest>{r2(bcdest)}</vBCFCPUFDest><pFCPUFDest>0.0000</pFCPUFDest>"
      f"<pICMSUFDest>20.5000</pICMSUFDest><pICMSInter>7.00</pICMSInter><pICMSInterPart>100.0000</pICMSInterPart>"
      f"<vFCPUFDest>0.00</vFCPUFDest><vICMSUFDest>{r2(difal)}</vICMSUFDest><vICMSUFRemet>0.00</vICMSUFRemet></ICMSUFDest>")
it = item_xml(1, {"cProd": "P001", "xProd": "NOTEBOOK 15 POL", "NCM": "84713012", "CFOP": "6108", "q": 1, "vUn": v, "vProd": v},
              f"<ICMS00><orig>0</orig><CST>00</CST><modBC>3</modBC><vBC>{r2(v)}</vBC><pICMS>7.0000</pICMS><vICMS>70.00</vICMS></ICMS00>", ufdest=uf)
nfe("nfe_venda_difal_ok.xml", "SP", emp,
    {"CPF": CPF_CLIENTE, "xNome": "CONSUMIDOR FINAL BA", "cMun": "2927408", "xMun": "Salvador", "UF": "BA", "indIEDest": "9"},
    "VENDA A NAO CONTRIBUINTE", [it], tot_base(v, 70, v, it[1], it[2], difal=difal), nnf=101, indfinal="1")

# 2) Venda com ST para PR com erros (sem CEST, MVA defasada, total incorreto, DIFAL ausente não se aplica)
v = 500.0
mva = 40.0
bst = round(v * 1.4, 2)
st = round(bst * 0.195 - v * 0.12, 2)
it = item_xml(1, {"cProd": "AUTO-77", "xProd": "PASTILHA DE FREIO", "NCM": "87083090", "CFOP": "6403", "q": 10, "vUn": 50, "vProd": v},
              f"<ICMS10><orig>0</orig><CST>10</CST><modBC>3</modBC><vBC>{r2(v)}</vBC><pICMS>12.0000</pICMS><vICMS>60.00</vICMS>"
              f"<modBCST>4</modBCST><pMVAST>{mva:.4f}</pMVAST><vBCST>{r2(bst)}</vBCST><pICMSST>19.5000</pICMSST><vICMSST>{r2(st)}</vICMSST></ICMS10>")
t = tot_base(v + 50, 60, v, it[1], it[2], bst, st)  # vProd incorreto de propósito
nfe("nfe_st_com_erros.xml", "SP", emp,
    {"CNPJ": CLIENTE_PR, "xNome": "AUTOPECAS CURITIBA LTDA", "cMun": "4106902", "xMun": "Curitiba", "UF": "PR", "indIEDest": "1", "IE": CLIENTE_PR_IE},
    "VENDA MERCADORIA COM ST", [it], t, nnf=102)

# 3) Entrada de fornecedor RS (para conciliação com pedido 4500012345)
forn = {"CNPJ": FORNEC_RS, "xNome": "INDUSTRIA GAUCHA DE EMBALAGENS SA", "cMun": "4314902", "xMun": "Porto Alegre", "UF": "RS", "IE": FORNEC_RS_IE, "CRT": "3"}
dest_emp = {"CNPJ": EMPRESA, "xNome": emp["xNome"], "cMun": "3550308", "xMun": "Sao Paulo", "UF": "SP", "indIEDest": "1", "IE": EMPRESA_IE}
itens = []
vbc = vic = vp = vc = vipi = vt = 0
for n, (cod, desc, ncm, q, vun, ped) in enumerate([("CX-100", "CAIXA PAPELAO 40X30", "48191000", 120, 3.5, "1"),
                                                     ("FITA-48", "FITA ADESIVA 48MM", "39191010", 60, 7.9, "2"),
                                                     ("PAL-01", "PALETE MADEIRA", "44152000", 4, 89.0, "3")], 1):
    vv = round(q * vun, 2)
    ii = item_xml(n, {"cProd": cod, "xProd": desc, "NCM": ncm, "CFOP": "6101", "q": q, "vUn": vun, "vProd": vv},
                  f"<ICMS00><orig>0</orig><CST>00</CST><modBC>3</modBC><vBC>{r2(vv)}</vBC><pICMS>12.0000</pICMS><vICMS>{r2(vv*0.12)}</vICMS></ICMS00>",
                  ipi=("50", 5.0), extra_prod=f"<xPed>4500012345</xPed><nItemPed>{ped}</nItemPed>")
    itens.append(ii)
    vt += vv; vbc += vv; vic += round(vv * 0.12, 2); vp += ii[1]; vc += ii[2]; vipi += round(vv * 0.05, 2)
nfe("nfe_entrada_fornecedor_rs.xml", "RS", forn, dest_emp, "VENDA PRODUCAO DO ESTABELECIMENTO", itens,
    tot_base(vt, vic, vbc, vp, vc, vipi=vipi), nnf=55871)

# 4) NF-e em contingência SVC-RS (UF SP deveria usar SVC-AN) sem protocolo
v = 250.0
it = item_xml(1, {"cProd": "P010", "xProd": "CADEIRA ESCRITORIO", "NCM": "94013090", "CFOP": "5102", "q": 1, "vUn": v, "vProd": v},
              f"<ICMS00><orig>0</orig><CST>00</CST><modBC>3</modBC><vBC>{r2(v)}</vBC><pICMS>18.0000</pICMS><vICMS>45.00</vICMS></ICMS00>")
t = tot_base(v, 45, v, it[1], it[2])
t["_cont"] = f"<dhCont>{(AGORA - timedelta(hours=30)).isoformat()}</dhCont><xJust>SEFAZ SP INDISPONIVEL - SERVICO PARALISADO</xJust>"
nfe("nfe_contingencia_svc.xml", "SP", emp,
    {"CNPJ": dv_cnpj("998877660001"), "xNome": "CLIENTE LOCAL SP", "cMun": "3550308", "xMun": "Sao Paulo", "UF": "SP", "indIEDest": "1", "IE": ie_sp("12345678", "11")},
    "VENDA", [it], t, tpemis="7", iddest="1", prot=False, nnf=103, dh=AGORA - timedelta(hours=30))

# 5) Devolução sem referência e com CFOP errado
v = 120.0
it = item_xml(1, {"cProd": "CX-100", "xProd": "CAIXA PAPELAO 40X30", "NCM": "48191000", "CFOP": "6102", "q": 20, "vUn": 6, "vProd": v},
              f"<ICMS00><orig>0</orig><CST>00</CST><modBC>3</modBC><vBC>{r2(v)}</vBC><pICMS>12.0000</pICMS><vICMS>14.40</vICMS></ICMS00>")
nfe("nfe_devolucao_incorreta.xml", "SP", emp, {"CNPJ": FORNEC_RS, "xNome": forn["xNome"], "cMun": "4314902", "xMun": "Porto Alegre", "UF": "RS", "indIEDest": "1", "IE": FORNEC_RS_IE},
    "DEVOLUCAO DE COMPRA", [it], tot_base(v, 14.4, v, it[1], it[2]), fin="4", nnf=104)

# 6) NFS-e Padrão Nacional tomada (limpeza – 7.10; retenções)
vs = 8000.0
nfse = f'''<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00"><infNFSe Id="NFS35503082{PRESTADOR}000000000000412{AGORA:%y%m}1234567890">
<xLocEmi>Sao Paulo</xLocEmi><xLocPrestacao>Sao Paulo</xLocPrestacao><nNFSe>412</nNFSe><cLocIncid>3550308</cLocIncid><xLocIncid>Sao Paulo</xLocIncid>
<xTribNac>Limpeza, manutencao e conservacao</xTribNac><verAplic>1.0</verAplic><ambGer>2</ambGer><tpEmis>1</tpEmis><procEmi>1</procEmi><cStat>100</cStat>
<dhProc>{AGORA.isoformat()}</dhProc><nDFSe>1</nDFSe>
<emit><CNPJ>{PRESTADOR}</CNPJ><xNome>LIMPA BEM SERVICOS LTDA</xNome><enderNac><xLgr>Rua C</xLgr><nro>3</nro><xBairro>Centro</xBairro><cMun>3550308</cMun><UF>SP</UF><CEP>01000000</CEP></enderNac></emit>
<valores><vBC>{r2(vs)}</vBC><pAliqAplic>2.00</pAliqAplic><vISSQN>160.00</vISSQN><vTotalRet>{r2(80+52+240+80)}</vTotalRet><vLiq>{r2(vs-80-52-240-80)}</vLiq></valores>
<DPS versao="1.00"><infDPS Id="DPS1"><tpAmb>2</tpAmb><dhEmi>{AGORA.isoformat()}</dhEmi><verAplic>1.0</verAplic><serie>1</serie><nDPS>412</nDPS><dCompet>{AGORA:%Y-%m-%d}</dCompet>
<tpEmit>1</tpEmit><cLocEmi>3550308</cLocEmi>
<prest><CNPJ>{PRESTADOR}</CNPJ><regTrib><opSimpNac>1</opSimpNac><regEspTrib>0</regEspTrib></regTrib></prest>
<toma><CNPJ>{EMPRESA}</CNPJ><xNome>{emp["xNome"]}</xNome></toma>
<serv><locPrest><cLocPrestacao>3550308</cLocPrestacao></locPrest><cServ><cTribNac>071001</cTribNac><xDescServ>Limpeza predial mensal</xDescServ></cServ></serv>
<valores><vServPrest><vServ>{r2(vs)}</vServ></vServPrest><trib><tribMun><tribISSQN>1</tribISSQN><tpRetISSQN>1</tpRetISSQN><pAliq>2.00</pAliq></tribMun>
<tribFed><piscofins><CST>01</CST><vBCPisCofins>{r2(vs)}</vBCPisCofins><pAliqPis>0.65</pAliqPis><pAliqCofins>3.00</pAliqCofins><vPis>52.00</vPis><vCofins>240.00</vCofins><tpRetPisCofins>1</tpRetPisCofins></piscofins>
<vRetIRRF>80.00</vRetIRRF><vRetCSLL>80.00</vRetCSLL></tribFed><totTrib><indTotTrib>0</indTotTrib></totTrib></trib></valores></infDPS></DPS></infNFSe></NFSe>'''
with open(os.path.join(OUT, "nfse_nacional_tomada.xml"), "w", encoding="utf-8") as f:
    f.write(nfse)

# 7) CT-e de frete sobre a compra do fornecedor RS
dh = AGORA - timedelta(hours=5)
chc = chave("RS", dh, TRANSP, "57", 1, 9001, "1", 77700001)
cte = f'''<?xml version="1.0" encoding="UTF-8"?>
<cteProc xmlns="http://www.portalfiscal.inf.br/cte" versao="4.00"><CTe><infCte versao="4.00" Id="CTe{chc}">
<ide><cUF>43</cUF><cCT>77700001</cCT><CFOP>6353</CFOP><natOp>PRESTACAO DE SERVICO DE TRANSPORTE</natOp><mod>57</mod><serie>1</serie><nCT>9001</nCT>
<dhEmi>{dh.isoformat()}</dhEmi><tpImp>1</tpImp><tpEmis>1</tpEmis><cDV>{chc[-1]}</cDV><tpAmb>2</tpAmb><tpCTe>0</tpCTe><procEmi>0</procEmi><verProc>1</verProc>
<cMunEnv>4314902</cMunEnv><xMunEnv>Porto Alegre</xMunEnv><UFEnv>RS</UFEnv><modal>01</modal><tpServ>0</tpServ><cMunIni>4314902</cMunIni><xMunIni>Porto Alegre</xMunIni><UFIni>RS</UFIni>
<cMunFim>3550308</cMunFim><xMunFim>Sao Paulo</xMunFim><UFFim>SP</UFFim><retira>1</retira><indIEToma>1</indIEToma><toma3><toma>3</toma></toma3></ide>
<emit><CNPJ>{TRANSP}</CNPJ><IE>{ie_rs("096654321")}</IE><xNome>TRANSPORTES PAMPA LTDA</xNome><enderEmit><cMun>4314902</cMun><xMun>Porto Alegre</xMun><UF>RS</UF></enderEmit></emit>
<rem><CNPJ>{FORNEC_RS}</CNPJ><xNome>{forn["xNome"]}</xNome></rem><dest><CNPJ>{EMPRESA}</CNPJ><xNome>{emp["xNome"]}</xNome></dest>
<vPrest><vTPrest>380.00</vTPrest><vRec>380.00</vRec></vPrest>
<imp><ICMS><ICMS00><CST>00</CST><vBC>380.00</vBC><pICMS>12.00</pICMS><vICMS>45.60</vICMS></ICMS00></ICMS></imp>
<infCTeNorm><infCarga><vCarga>1592.40</vCarga><proPred>EMBALAGENS</proPred></infCarga><infDoc><infNFe><chave>__CHAVE_NFE__</chave></infNFe></infDoc></infCTeNorm>
</infCte></CTe><protCTe versao="4.00"><infProt><tpAmb>2</tpAmb><chCTe>{chc}</chCTe><dhRecbto>{dh.isoformat()}</dhRecbto><nProt>143260000009001</nProt><cStat>100</cStat><xMotivo>Autorizado o uso do CT-e</xMotivo></infProt></protCTe></cteProc>'''
ent = open(os.path.join(OUT, "nfe_entrada_fornecedor_rs.xml"), encoding="utf-8").read()
ch_ent = ent.split('Id="NFe')[1][:44]
with open(os.path.join(OUT, "cte_frete_compra.xml"), "w", encoding="utf-8") as f:
    f.write(cte.replace("__CHAVE_NFE__", ch_ent))

print("Empresa demo:", EMPRESA, "IE", EMPRESA_IE)
print("Fornecedor RS:", FORNEC_RS)
print("Amostras geradas em", OUT)

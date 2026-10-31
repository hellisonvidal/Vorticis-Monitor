package xmlfiscal

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Tipo identifica o tipo de documento fiscal detectado.
type Tipo string

const (
	TipoNFe    Tipo = "NFE"   // modelo 55
	TipoNFCe   Tipo = "NFCE"  // modelo 65
	TipoCTe    Tipo = "CTE"   // modelo 57/67
	TipoMDFe   Tipo = "MDFE"  // modelo 58
	TipoNFSe   Tipo = "NFSE"
	TipoEvento Tipo = "EVENTO"
	TipoResumo Tipo = "RESUMO_NFE"
)

// Documento é o resultado do parse de qualquer XML suportado.
type Documento struct {
	Tipo    Tipo
	NFe     *InfNFe
	ProtNFe *ProtNFe
	CTe     *CTeProc
	MDFe    *MDFeProc
	NFSe    *NFSe
	Evento  *ProcEvento
	Resumo  *ResNFe
	Bruto   []byte
}

// RaizXML devolve o nome do elemento raiz.
func RaizXML(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

func decode(data []byte, v any) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	return dec.Decode(v)
}

// ErrNaoSuportado indica XML não reconhecido.
var ErrNaoSuportado = errors.New("xml não reconhecido como documento fiscal suportado")

// Parse detecta o tipo e decodifica o documento.
func Parse(data []byte) (*Documento, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	raiz, err := RaizXML(data)
	if err != nil {
		return nil, fmt.Errorf("xml inválido: %w", err)
	}
	d := &Documento{Bruto: data}
	switch raiz {
	case "nfeProc":
		var p NFeProc
		if err := decode(data, &p); err != nil {
			return nil, err
		}
		d.NFe, d.ProtNFe = &p.NFe.InfNFe, p.ProtNFe
	case "NFe":
		var n NFe
		if err := decode(data, &n); err != nil {
			return nil, err
		}
		d.NFe = &n.InfNFe
	case "cteProc", "CTe", "cteOSProc", "CTeOS":
		var c CTeProc
		if raiz == "CTe" || raiz == "CTeOS" {
			var inner struct {
				InfCte InfCte `xml:"infCte"`
			}
			if err := decode(data, &inner); err != nil {
				return nil, err
			}
			c.CTe.InfCte = inner.InfCte
		} else if err := decode(data, &c); err != nil {
			return nil, err
		}
		d.Tipo, d.CTe = TipoCTe, &c
		return d, nil
	case "mdfeProc":
		var m MDFeProc
		if err := decode(data, &m); err != nil {
			return nil, err
		}
		d.Tipo, d.MDFe = TipoMDFe, &m
		return d, nil
	case "NFSe":
		var n nfseNacional
		if err := decode(data, &n); err != nil {
			return nil, err
		}
		x := n.Normaliza()
		d.Tipo, d.NFSe = TipoNFSe, &x
		return d, nil
	case "CompNfse", "Nfse", "ConsultarNfseResposta", "GerarNfseResposta":
		inf, err := extraiInfNfse(data)
		if err != nil {
			return nil, err
		}
		x := inf.Normaliza()
		d.Tipo, d.NFSe = TipoNFSe, &x
		return d, nil
	case "procEventoNFe", "procEventoCTe":
		var e ProcEvento
		if err := decode(data, &e); err != nil {
			return nil, err
		}
		d.Tipo, d.Evento = TipoEvento, &e
		return d, nil
	case "resNFe":
		var r ResNFe
		if err := decode(data, &r); err != nil {
			return nil, err
		}
		d.Tipo, d.Resumo = TipoResumo, &r
		return d, nil
	default:
		return nil, fmt.Errorf("%w (raiz <%s>)", ErrNaoSuportado, raiz)
	}
	if d.NFe.Ide.Mod == "65" {
		d.Tipo = TipoNFCe
	} else {
		d.Tipo = TipoNFe
	}
	return d, nil
}

// extraiInfNfse localiza o primeiro <InfNfse> em qualquer envelope ABRASF.
func extraiInfNfse(data []byte) (nfseABRASF, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	var out nfseABRASF
	for {
		tok, err := dec.Token()
		if err != nil {
			return out, fmt.Errorf("InfNfse não encontrado: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && strings.EqualFold(se.Name.Local, "InfNfse") {
			return out, dec.DecodeElement(&out, &se)
		}
	}
}

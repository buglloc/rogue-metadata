package blackhole

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	listeners []*dns.Server
	handler   *dns.ServeMux
	log       zerolog.Logger
}

type Config struct {
	Listen   string
	Upstream string
	IDP      string
	Names    []string
	IPs      []string
}

func NewServer(cfg Config) (*Server, error) {
	srv := &Server{
		listeners: make([]*dns.Server, 2),
		handler:   dns.NewServeMux(),
		log:       log.With().Str("source", "blackhole.dns").Logger(),
	}

	if err := srv.buildHandler(cfg); err != nil {
		return nil, fmt.Errorf("failed to build DNS handler: %w", err)
	}

	for i, n := range []string{"tcp", "udp"} {
		n := n
		srv.listeners[i] = &dns.Server{
			Addr: cfg.Listen,
			Net:  n,
			NotifyStartedFunc: func() {
				srv.log.Info().
					Str("net", n).
					Str("addr", cfg.Listen).
					Msg("started")
			},
			Handler: srv.handler,
		}
	}

	return srv, nil
}

func (s *Server) ListenAndServe() error {
	g, _ := errgroup.WithContext(context.Background())

	for _, lis := range s.listeners {
		lis := lis
		g.Go(func() error {
			if err := lis.ListenAndServe(); err != nil {
				return fmt.Errorf("listener for net %q failed: %w", lis.Net, err)
			}

			return nil
		})
	}

	return g.Wait()
}

func (s *Server) Shutdown(ctx context.Context) error {
	var errs []error
	for _, l := range s.listeners {
		if err := l.ShutdownContext(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Server) buildHandler(cfg Config) error {
	if err := s.buildHolesRoutes(cfg.Names, cfg.IPs); err != nil {
		return fmt.Errorf("build black hole routes: %w", err)
	}

	if err := s.buildDataRoutes(cfg.IDP); err != nil {
		return fmt.Errorf("build data routes: %w", err)
	}

	if err := s.buildProxyHandler(cfg.Upstream); err != nil {
		return fmt.Errorf("build proxy routes: %w", err)
	}

	return nil
}

func (s *Server) buildHolesRoutes(zones []string, ips []string) error {
	v4RRs, v6RRs, err := ipsToRRs(ips)
	if err != nil {
		return fmt.Errorf("parse ips: %w", err)
	}

	for _, zone := range zones {
		s.queryHandler(zone, v4RRs, v6RRs)
	}

	return nil
}

func (s *Server) buildDataRoutes(cfgIP string) error {
	ips, err := s.dataIPs(cfgIP)
	if err != nil {
		return fmt.Errorf("collect data ips: %w", err)
	}

	s.queryHandler("instance-data.", ips, nil)
	return nil
}

func (s *Server) buildProxyHandler(upstream string) error {
	c := new(dns.Client)
	s.handler.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {
		rsp, _, err := c.Exchange(req, upstream)
		if err != nil {
			s.log.Warn().Err(err).Msg("proxy request failed")
			return
		}

		rsp.SetReply(req)
		_ = w.WriteMsg(rsp)
	})

	return nil
}

func (s *Server) dataIPs(cfgIP string) ([]dns.A, error) {
	if cfgIP != "" {
		ip := net.ParseIP(cfgIP)
		if isV4IP(ip) {
			return nil, fmt.Errorf("invalid IP in config: %s", cfgIP)
		}

		return []dns.A{{A: ip}}, nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("get ifaces: %w", err)
	}

	var out []dns.A
	for _, i := range ifaces {
		if !strings.HasPrefix(i.Name, "eth") && !strings.HasPrefix(i.Name, "eno") {
			s.log.Info().Str("iface", i.Name).Msg("skip iface")
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			s.log.Info().Err(err).Str("iface", i.Name).Msg("skip iface w/o addrs")
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}

			if !isV4IP(ip) {
				s.log.Warn().Str("iface", i.Name).Stringer("ip", ip).Msg("skip non-v4 IP")
				continue
			}

			if !ip.IsPrivate() {
				s.log.Warn().Str("iface", i.Name).Stringer("ip", ip).Msg("skip non-private IP")
				continue
			}

			s.log.Info().Str("iface", i.Name).Stringer("ip", ip).Msg("use IP as data instance")
			out = append(out, dns.A{
				A: ip,
			})
		}
	}

	return out, nil
}

func (s *Server) queryHandler(zone string, v4RRs []dns.A, v6RRs []dns.AAAA) {
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}

	s.handler.HandleFunc(zone, func(w dns.ResponseWriter, req *dns.Msg) {
		defer func() { _ = w.Close() }()

		out := &dns.Msg{}
		out.SetReply(req)

		for _, question := range req.Question {
			switch question.Qtype {
			case dns.TypeA:
				for _, ip := range v4RRs {
					s.log.Info().
						Stringer("client", w.RemoteAddr()).
						Msgf("%s -> %s", question.Name, ip.A)

					out.Answer = append(out.Answer, &dns.A{
						Hdr: dns.RR_Header{
							Name:   question.Name,
							Rrtype: question.Qtype,
							Class:  dns.ClassINET,
							Ttl:    90,
						},
						A: ip.A,
					})
				}

			case dns.TypeAAAA:
				for _, ip := range v6RRs {
					s.log.Info().
						Stringer("client", w.RemoteAddr()).
						Msgf("%s -> %s", question.Name, ip.AAAA)

					out.Answer = append(out.Answer, &dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   question.Name,
							Rrtype: question.Qtype,
							Class:  dns.ClassINET,
							Ttl:    90,
						},
						AAAA: ip.AAAA,
					})
				}
			}
		}

		_ = w.WriteMsg(out)
	})
}

func ipsToRRs(ips []string) (v4RRs []dns.A, v6RRs []dns.AAAA, err error) {
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed.IsUnspecified() {
			return nil, nil, fmt.Errorf("invalid IP: %s", ip)
		}

		switch {
		case isV4IP(parsed):
			v4RRs = append(v4RRs, dns.A{
				A: parsed,
			})

		default:
			v6RRs = append(v6RRs, dns.AAAA{
				AAAA: parsed,
			})
		}
	}

	return
}

func isV4IP(ip net.IP) bool {
	p4 := ip.To4()
	return len(p4) == net.IPv4len
}

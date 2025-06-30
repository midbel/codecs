<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:variable name="version" select="'0.1'" static="yes"/>
	<xsl:include href="commons-{$version}.xslt"/>
	<xsl:template match="/">
		<xsl:variable name="var" select="/root/language[1]"/>
		<language>
			<lang><xsl:value-of select="$var/@id"/></lang>
			<xsl:call-template name="foobar"/>
		</language>
	</xsl:template>
</xsl:stylesheet>
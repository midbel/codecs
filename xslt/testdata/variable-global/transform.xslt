<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:variable name="global" select="'angle'"/>
	<xsl:template match="/">
		<xsl:variable name="local" select="/root/language[1]"/>
		<language>
			<global>
				<xsl:value-of select="$global"/>
			</global>
			<local>
				<xsl:value-of select="$local/name"/>
			</local>
			<lang><xsl:value-of select="$local/@id"/></lang>
		</language>
	</xsl:template>
</xsl:stylesheet>